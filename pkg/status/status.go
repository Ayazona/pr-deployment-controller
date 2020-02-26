package status

import (
	"context"
	"html/template"
	"net/http"
	"os"

	"github.com/oklog/oklog/pkg/group"

	"github.com/kolonialno/pr-deployment-controller/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type windowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
	X    uint16
	Y    uint16
}

// Status contains the required controllers to serve the statuspage used during environment modifications
type Status struct {
	logger *logrus.Entry

	r *mux.Router

	restconfig   *rest.Config
	k8sEnv       *k8s.Environment
	corev1client *corev1client.CoreV1Client
}

// New creates a new instance of the status page server
func New(logger *logrus.Entry, restconfig *rest.Config, k8sEnv *k8s.Environment) (*Status, error) {
	r := mux.NewRouter()

	status := &Status{
		logger: logger,
		r:      r,

		restconfig: restconfig,
		k8sEnv:     k8sEnv,
	}

	err := status.createCoreV1Client()
	if err != nil {
		return nil, err
	}

	// Remote terminal for exec commands
	r.PathPrefix("/term/__static/").Handler(http.StripPrefix("/term/__static/", http.FileServer(http.Dir("./public"))))
	r.Path("/term/{build}/{container}/{cmd}/").HandlerFunc(status.terminalHandler)
	r.Path("/term/{build}/{container}/{cmd}/ws/").HandlerFunc(status.streamTerminal)

	// Generic statuspage before environment is ready
	r.PathPrefix("/").HandlerFunc(status.statusHandler)

	return status, nil
}

func (s *Status) createCoreV1Client() error {
	// Create a Kubernetes core/v1 client.
	coreclient, err := corev1client.NewForConfig(s.restconfig)
	if err != nil {
		return err
	}

	s.corev1client = coreclient

	return nil
}

func (s *Status) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	s.r.ServeHTTP(rw, r)
}

func (s *Status) statusHandler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl, _ := template.New("landingTemplate").Parse(landingTemplate)

	data := map[string]interface{}{}

	render(rw, r, tmpl, "landingTemplate", data)
}

func (s *Status) terminalHandler(rw http.ResponseWriter, r *http.Request) {
	// Load url vars
	vars := mux.Vars(r)

	// Load required commands
	build, ok := vars["build"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}
	container, ok := vars["container"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}
	cmd, ok := vars["cmd"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}

	// Lookup pod and entrypoint to execute
	_, _, err := getpod(s.k8sEnv, build, container, cmd)
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// We found a pod, return the terminal
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("terminalTemplate").Parse(terminalTemplate)
	data := map[string]interface{}{}

	render(rw, r, tmpl, "terminalTemplate", data)
}

func (s *Status) streamTerminal(rw http.ResponseWriter, r *http.Request) {
	// Load url vars
	vars := mux.Vars(r)

	// Load required commands
	build, ok := vars["build"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}
	container, ok := vars["container"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}
	cmd, ok := vars["cmd"]
	if !ok {
		errorHandler(rw, ErrMissingVar)
		return
	}

	// Lookup pod and entrypoint to execute
	pod, entrypoint, err := getpod(s.k8sEnv, build, container, cmd)
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// Pipe used for stdout
	outr, outw, err := os.Pipe()
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// Pipe used for stdin
	inr, inw, err := os.Pipe()
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// Prepare the API URL used to execute another process within the pod.
	req := s.corev1client.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
			Command:   entrypoint,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.restconfig, "POST", req.URL())
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// Upgrade connections
	ws, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		errorHandler(rw, err)
		return
	}

	// Initialize the size queue for terminal size changes
	sizeq := newSizeQueue()
	defer sizeq.Close()

	//
	// We're now ready to exec the remote command and stream
	// the pipes between the command and the terminal
	//

	var g group.Group
	{
		// Call kubernetes
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			err = stream(ctx, exec, NewReader(ctx, inr), NewWriter(ctx, outw), sizeq)
			s.logger.WithError(err).Warn("kubernetes stream failed")
			return err
		}, func(err error) {
			ws.Close() // nolint: errcheck, gas
			cancel()
		})
	}
	{
		// Redirect stdout to the ws connection
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			err = redirectStdout(ctx, NewReader(ctx, outr), ws)
			s.logger.WithError(err).Warn("stdout redirect stream failed")
			return err
		}, func(err error) {
			cancel()
		})
	}
	{
		// Redirect stdin to the ws connection
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			err = redirectStdin(ctx, NewWriter(ctx, inw), ws, sizeq)
			s.logger.WithError(err).Warn("stdin redirect stream failed")
			return err
		}, func(err error) {
			cancel()
		})
	}
	if err = g.Run(); err != nil {
		s.logger.WithError(err).Warn("terminal stream exited with error")
	}
}
