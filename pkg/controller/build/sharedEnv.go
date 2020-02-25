package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"text/template"

	"github.com/kolonialno/test-environment-manager/pkg/internal"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// nolint: gocyclo
func (br *buildReconciler) reconcileSharedEnv() error {
	name := fmt.Sprintf("%ssharedenv", options.BuildPrefix)
	logger := br.logger.WithField("configmap", name)

	var err error

	type props struct {
		Owner             string
		Repository        string
		PullRequestNumber int64
		Image             string
		ServerDomain      string
		Namespace         string
		Version           string

		// Options provided by the database template
		DatabaseName     string
		DatabaseUser     string
		DatabasePassword string
		DatabaseHost     string
		DatabasePort     string
	}

	p := props{
		Owner:             br.build.Spec.Git.Owner,
		Repository:        br.build.Spec.Git.Repository,
		PullRequestNumber: br.build.Spec.Git.PullRequestNumber,
		Image:             br.build.Spec.Image,
		ServerDomain: internal.GenerateBuildURL(
			br.build.Spec.Git.Owner,
			br.build.Spec.Git.Repository,
			br.build.Spec.Git.PullRequestNumber,
			options.ClusterDomain,
		),
		Namespace: fmt.Sprintf(
			"%s%s-%s-%d",
			options.BuildPrefix,
			br.build.Spec.Git.Owner,
			br.build.Spec.Git.Repository,
			br.build.Spec.Git.PullRequestNumber,
		),
		Version: br.build.Spec.Git.Ref,
	}

	// Retrieve database claim
	var dc *databaseclaim
	dc, err = newDatabaseClaim(br)
	if err != nil {
		return err
	}

	dbopts, err := dc.claim(br.ctx)
	if err != nil {
		return err
	} else if dbopts != nil {
		p.DatabaseName = dbopts.Name
		p.DatabaseUser = dbopts.Username
		p.DatabasePassword = dbopts.Password
		p.DatabaseHost = dbopts.Host
		p.DatabasePort = strconv.Itoa(int(dbopts.Port))
	}

	data := map[string]string{}
	for _, envSpec := range br.environment.Spec.SharedEnv {
		var tmpl *template.Template

		tmpl, err = template.New("env").Parse(envSpec.Value)
		if err != nil {
			return err
		}

		buff := bytes.NewBufferString("")

		err = tmpl.Execute(buff, p)
		if err != nil {
			return err
		}

		var value []byte

		value, err = ioutil.ReadAll(buff)
		if err != nil {
			return err
		}

		data[envSpec.Name] = string(value)
	}

	deploy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: br.namespace,
		},
		Data: data,
	}
	if err = controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &corev1.ConfigMap{}
	err = br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating configmap")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
