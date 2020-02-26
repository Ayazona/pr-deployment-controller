package status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gorilla/websocket"
	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/pr-deployment-controller/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getpod tries to get a pod and the command to execute based on the given params
// nolint: gocyclo
func getpod(k *k8s.Environment, build, container, cmd string) (*corev1.Pod, []string, error) {
	ctx := context.Background()

	foundbuild := &testenvironmentv1alpha1.Build{}
	err := k.Get(ctx, types.NamespacedName{Name: build, Namespace: k.Namespace}, foundbuild)
	if err != nil {
		return nil, nil, err
	}

	foundenvironment := &testenvironmentv1alpha1.Environment{}
	err = k.Get(ctx, types.NamespacedName{Name: foundbuild.Spec.Environment, Namespace: k.Namespace}, foundenvironment)
	if err != nil {
		return nil, nil, err
	}

	var remoteCommand *testenvironmentv1alpha1.ExecSpec
	for _, containerdef := range foundenvironment.Spec.Containers {
		if containerdef.Name == container {
			for _, remoteterminal := range containerdef.RemoteTerminal {
				remoteterminal := remoteterminal
				if remoteterminal.Name == cmd {
					remoteCommand = &remoteterminal
					break
				}
			}
		}
	}

	if remoteCommand == nil {
		return nil, nil, ErrUnknownCommand
	}

	pods := &corev1.PodList{}
	err = k.List(ctx, &client.ListOptions{Namespace: fmt.Sprintf(
		"%s%s-%s-%d",
		k.BuildPrefix,
		foundbuild.Spec.Git.Owner,
		foundbuild.Spec.Git.Repository,
		foundbuild.Spec.Git.PullRequestNumber,
	)}, pods)
	if err != nil {
		return nil, nil, err
	}

	for _, pod := range pods.Items {
		pod := pod
		if strings.HasPrefix(pod.Name, fmt.Sprintf("%s-container", container)) {
			// Make sure the pod is running
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return &pod, remoteCommand.Cmd, nil
				}
			}
		}
	}

	return nil, nil, ErrNoPodFound
}

func stream(
	ctx context.Context,
	exec remotecommand.Executor,
	stdin io.Reader,
	stdout io.Writer,
	s remotecommand.TerminalSizeQueue,
) error {
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stdout,
		Tty:               true,
		TerminalSizeQueue: s,
	})
}

func redirectStdout(ctx context.Context, stdout io.Reader, conn *websocket.Conn) error {
	for {
		buf := make([]byte, 1024)
		read, err := stdout.Read(buf)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.BinaryMessage, buf[:read])
		if err != nil {
			return err
		}
	}
}

// nolint: gocyclo
func redirectStdin(ctx context.Context, stdin io.Writer, conn *websocket.Conn, s *sizeQueue) error {
	for {
		messageType, reader, err := conn.NextReader()
		if err != nil {
			return err
		}

		if messageType == websocket.TextMessage {
			conn.WriteMessage(websocket.TextMessage, []byte("Unexpected text message")) // nolint: gas, errcheck
			continue
		}

		dataTypeBuf := make([]byte, 1)
		read, err := reader.Read(dataTypeBuf)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte("Unable to read message type from reader")) // nolint: gas, errcheck
			return err
		}

		if read != 1 {
			return errors.New("empty reader")
		}

		switch dataTypeBuf[0] {
		case 0:
			_, err := io.Copy(stdin, reader)
			if err != nil {
				return err
			}
		case 1:
			decoder := json.NewDecoder(reader)
			resizeMessage := windowSize{}
			err := decoder.Decode(&resizeMessage)
			if err != nil {
				conn.WriteMessage( // nolint: errcheck
					websocket.TextMessage, []byte("Error decoding resize message: "+err.Error()),
				)
				continue
			}

			s.Resize(remotecommand.TerminalSize{
				Width:  resizeMessage.Cols,
				Height: resizeMessage.Rows,
			})
		default:
			return errors.New("unknown data type")
		}
	}
}
