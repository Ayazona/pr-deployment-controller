package cleanup

import (
	"context"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/test-environment-manager/pkg/github"
	"github.com/kolonialno/test-environment-manager/pkg/internal"
	"github.com/kolonialno/test-environment-manager/pkg/k8s"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cleanup defines the service responsible for cleaning up old test-environments.
type Cleanup interface {
	internal.Service
}

type baseCleanup struct {
	stop chan struct{}

	logger *logrus.Entry
	k8s    *k8s.Environment
	github github.Github
}

// New creates a new instance of the cleanup worker
func New(logger *logrus.Entry, k8s *k8s.Environment, github github.Github) (Cleanup, error) {
	cleanup := &baseCleanup{
		stop: make(chan struct{}, 1),

		logger: logger,
		k8s:    k8s,
		github: github,
	}

	return cleanup, nil
}

func (c *baseCleanup) Runnable() (internal.RunFunc, internal.StopFunc) {
	return c.Run, c.Stop
}

func (c *baseCleanup) Run() error {
	ticker := time.NewTicker(IterationDelay)

	for {
		select {
		case <-ticker.C:
			err := c.cleanup()
			if err != nil {
				return err
			}
		case <-c.stop:
			return nil
		}
	}
}

func (c *baseCleanup) Stop(err error) {
	if err != nil {
		c.logger.WithError(err).Warn("stopping cleanup worker due to error")
	}

	c.stop <- struct{}{}
}

// cleanup loops over the build instances inside the operator namespace and
// deletes resources older than EnvironmentLifetime
func (c *baseCleanup) cleanup() error {
	ctx := context.TODO()

	builds := &testenvironmentv1alpha1.BuildList{}

	err := c.k8s.List(ctx, &client.ListOptions{Namespace: c.k8s.Namespace}, builds)
	if err != nil {
		c.logger.WithError(err).Warn("could not lookup builds")
		return nil
	}

	c.logger.WithField("buildCount", len(builds.Items)).Info("scanning for stale builds")

	for _, build := range builds.Items {
		build := build
		created := build.ObjectMeta.CreationTimestamp.Time
		oldest := time.Now().Add(-EnvironmentLifetime)

		// Delete evironment if the creation time is before the oldest allowed time
		if created.Before(oldest) {
			logger := c.logger.WithFields(logrus.Fields{
				"build":       build.Name,
				"environment": build.Spec.Environment,
				"repository":  build.Spec.Git.Repository,
				"owner":       build.Spec.Git.Owner,
				"age":         time.Since(created).String(),
			})
			logger.Info("old build detected")

			// Delete environment
			err := c.k8s.Delete(ctx, &build)
			if err != nil {
				logger.WithError(err).Warn("could not delete build")
				continue
			}

			// Update github status
			c.github.PostBuildStatus( // nolint: errcheck, gas
				ctx,
				build.Spec.Git.Owner,
				build.Spec.Git.Repository,
				build.Spec.Git.Ref,
				github.SuccessState,
				"Environment closed (no activity last 48h)",
				"",
			)
		}
	}

	return nil
}
