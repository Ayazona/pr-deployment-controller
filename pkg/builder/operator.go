package builder

import (
	"context"
	"fmt"
	"reflect"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Get the environment definition for a build environment
func (w *worker) getEnvironmentManifest(
	ctx context.Context,
	owner,
	repository string,
) (*testenvironmentv1alpha1.Environment, error) {
	environment := &testenvironmentv1alpha1.Environment{}

	err := w.options.K8s.Get(ctx, types.NamespacedName{
		Name:      environmentName(owner, repository),
		Namespace: w.options.K8s.Namespace,
	}, environment)

	return environment, err
}

// buildExists returns true if the build already exists in the cluster
func (w *worker) buildExists(ctx context.Context, j *job) (bool, error) {
	found := &testenvironmentv1alpha1.Build{}

	err := w.options.K8s.Get(
		ctx,
		types.NamespacedName{
			Name:      environmentBuildName(j.owner, j.repository, j.pullRequestNumber),
			Namespace: w.options.K8s.Namespace,
		},
		found,
	)
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// create a build manifest, used to create/update a test environment
func (w *worker) createBuildManifest(
	ctx context.Context,
	env string,
	j *job,
	imageName string,
) error {
	build := &testenvironmentv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      environmentBuildName(j.owner, j.repository, j.pullRequestNumber),
			Namespace: w.options.K8s.Namespace,
		},
		Spec: testenvironmentv1alpha1.BuildSpec{
			Environment: env,
			Image:       imageName,
			Git: &testenvironmentv1alpha1.GitSpec{
				Owner:             j.owner,
				Repository:        j.repository,
				Ref:               j.ref,
				PullRequestNumber: j.pullRequestNumber,
			},
		},
	}

	found := &testenvironmentv1alpha1.Build{}

	err := w.options.K8s.Get(
		ctx,
		types.NamespacedName{Name: build.ObjectMeta.Name, Namespace: build.ObjectMeta.Namespace},
		found,
	)
	if err != nil && errors.IsNotFound(err) {
		return w.options.K8s.Create(ctx, build)
	} else if err != nil {
		return err
	}

	if !reflect.DeepEqual(build.Spec, found.Spec) {
		found.Spec = build.Spec
		return w.options.K8s.Update(ctx, found)
	}

	return nil
}

// Delete existing build manifest if found, used to remove a test environment
func (w *worker) deleteBuildManifest(ctx context.Context, j *job) error {
	err := w.options.K8s.Delete(ctx, &testenvironmentv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      environmentBuildName(j.owner, j.repository, j.pullRequestNumber),
			Namespace: w.options.K8s.Namespace,
		},
	})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}

	return err
}

// Get the environment name based on the git owner/repository values
func environmentName(owner, repository string) string {
	return fmt.Sprintf("%s-%s", owner, repository)
}

// Get the build name based on the git owner/repository values
func environmentBuildName(owner, repository string, pullRequestNumber int64) string {
	return fmt.Sprintf("%s-%s-%d", owner, repository, pullRequestNumber)
}
