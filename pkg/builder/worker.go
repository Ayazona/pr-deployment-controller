package builder

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/pr-deployment-controller/pkg/github"
	"github.com/kolonialno/pr-deployment-controller/pkg/internal"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type worker struct {
	id     int
	logger *logrus.Entry

	jobs    <-chan *job
	results chan<- *jobResult

	scheduler *scheduler
	options   *Options
}

// newWorker creates a new worker (used to build and push docker images)
func newWorker(
	id int,
	logger *logrus.Entry,
	jobs <-chan *job,
	results chan<- *jobResult,
	scheduler *scheduler,
	options *Options,
) (*worker, error) {
	return &worker{
		id:     id,
		logger: logger,

		jobs:    jobs,
		results: results,

		scheduler: scheduler,
		options:   options,
	}, nil
}

// run starts to consume and process jobs
func (w *worker) run() {
	for {
		j, more := <-w.jobs
		if more {
			err := w.processJob(j)
			w.results <- &jobResult{job: j, err: err}
		} else {
			w.logger.Info("received all jobs, closing worker")
			return
		}
	}
}

func (w *worker) processJob(j *job) error {
	ctx := context.Background()

	var currentTime = time.Now()
	j.startTime = &currentTime

	// Track queue delay
	w.options.RuntimeSummary.WithLabelValues(
		j.owner,
		j.repository,
		strconv.FormatInt(j.pullRequestNumber, 10),
		strconv.FormatInt(j.id, 10),
		"queue_delay",
	).Observe(currentTime.Sub(*j.createTime).Seconds())

	if j.deleteEnvironment {
		return w.deleteBuild(ctx, j)
	}

	return w.createBuild(ctx, j)
}

// nolint: gocyclo
func (w *worker) createBuild(ctx context.Context, j *job) error {
	// Dockerfile path inside the build context
	dockerFile := "Dockerfile"
	// imageName to use then building an pushing the docker image, contains the full registry path
	imageName := w.options.Docker.ImageName(j.owner, j.repository, j.ref)
	// Initialize logger for the createBuild task
	logger := w.logger.WithFields(log.Fields{
		"job_id":              j.id,
		"owner":               j.owner,
		"repository":          j.repository,
		"pull_request_number": j.pullRequestNumber,
		"ref":                 j.ref,
		"user":                j.user,
		"firstRun":            j.firstRun,
		"clean":               j.clean,
		"force":               j.force,
	})
	logger.Info("creating build")

	//
	// Helper methods for processing a build job
	//

	// executeFunction executes a function and logs/tracks the execution
	executeFunction := func(f func() error, operation, description string) error {
		// Log event to stdout
		logger.WithField("operation", operation).Info(strings.ToLower(description))

		// Notify Github about the operation
		w.updateBuildStatus( // nolint: gas, errcheck
			ctx, j, github.PendingState, description, "",
		)

		// Execute f and track runtime
		startTime := time.Now()
		err := f()
		w.trackTask(j, operation, startTime)

		// Return the function err
		return err
	}

	// checkError functions as a helper for checking an error and update commit
	// status / log error message if something is wrong
	checkError := func(err error, errorMessage string) bool {
		if err != nil {
			// Log error to stdout
			logger.WithError(err).Error(strings.ToLower(errorMessage))

			//Update commit status
			w.updateBuildStatus( // nolint: gas, errcheck
				ctx, j, github.ErrorState, errorMessage, "",
			)

			// Error observed, return true
			return true
		}

		return false
	}

	//
	// Process the actual build
	//

	var err error
	var repositoryArchive io.ReadCloser
	var buildContext io.Reader
	var environment *testenvironmentv1alpha1.Environment

	// Cleanup after return
	defer func() {
		if repositoryArchive != nil {
			repositoryArchive.Close() // nolint: errcheck
		}
	}()

	// Check if the environment exists
	err = executeFunction(func() error {
		environment, err = w.getEnvironmentManifest(ctx, j.owner, j.repository)
		return err
	}, "checkManifest", "Environment manifest lookup")
	if checkError(err, "Unknown environment") {
		return err
	}

	// Skip the build if IgnoredUser contains the commit user and this
	// is not a forced build.
	err = executeFunction(func() error {
		// Forced build
		if j.force {
			return nil
		}

		// Ignore build if the commit user exists in IgnoredUsers
		for _, ignoredUser := range environment.Spec.IgnoredUsers {
			if ignoredUser == j.user {
				return ErrJobIgnored
			}
		}

		return nil
	}, "checkIgnoredUsers", "Checking ignored users against the commit user")
	if err == ErrJobIgnored {
		// Log error to stdout
		logger.Warn("Job ignored, user in IgnoredUsers")

		//Update commit status
		w.updateBuildStatus( // nolint: gas, errcheck
			ctx, j, github.SuccessState, "Build ignored (ignoring commits from this user)", "",
		)

		return nil
	} else if checkError(err, "Could not lookup ignored users") {
		return err
	}

	// Clone repository
	err = executeFunction(func() error {
		repositoryArchive, err = w.options.GitHub.CloneBuild(ctx, j.owner, j.repository, j.ref)
		return err
	}, "cloneRepository", "Cloning repository")
	if checkError(err, "Could not clone repository") {
		return err
	}

	// Process build context
	err = executeFunction(func() error {
		buildContext, err = w.processBuildContext(j, dockerFile, repositoryArchive)
		return err
	}, "processingBuildContext", "Processing build context")
	if checkError(err, "Could not process build context") {
		return err
	}

	// Building image
	err = executeFunction(func() error {
		return w.options.Docker.BuildImage(ctx, buildContext, imageName, dockerFile)
	}, "buildImage", "Building image")
	if checkError(err, "Could not build image") {
		return err
	}

	// Pushing image
	err = executeFunction(func() error {
		return w.options.Docker.PushImage(ctx, imageName)
	}, "pushImage", "Pushing image to remote registry")
	if checkError(err, "Could not push image to remote registry") {
		return err
	}

	// Skip build if the environment is configured as an on demand environment
	// Continue if this is a forced build
	buildExists, err := w.buildExists(ctx, j)
	if checkError(err, "Could not lookup existing build manifest") {
		return err
	}
	if environment.Spec.OnDemand && !j.force && !buildExists {
		// Comment on the PR (Post test-environment information)
		if j.firstRun {
			w.commentEnvironmentInformation(ctx, j, environment) // nolint: gas, errcheck
		}

		// The build finished successfully, post success state to Github
		w.updateBuildStatus( // nolint: gas, errcheck
			ctx,
			j,
			github.SuccessState,
			"Build ready, comment /rebuild to deploy the latest commit",
			"",
		)

		return nil
	}

	// Make sure the job ID is higher than the previous job for this repository
	err = w.scheduler.scheduleJob(fmt.Sprintf("%s/%s-%d", j.owner, j.repository, j.pullRequestNumber), j.id)
	if checkError(err, "Build outdated") {
		return err
	}

	// Delete the old build manifest if the clean parameter is set (new build with clean db)
	if j.clean {
		err = executeFunction(func() error {
			return w.deleteBuildManifest(ctx, j)
		}, "deleteBuildManifest", "Deleting old build manifest (clean environment)")
		if checkError(err, "Could not delete old build manifest") {
			return err
		}
	}

	// Create build manifest
	err = executeFunction(func() error {
		return w.createBuildManifest(ctx, environment.ObjectMeta.Name, j, imageName)
	}, "createBuildManifest", "Creating build manifest")
	if checkError(err, "Could not create build manifest") {
		return err
	}

	// Comment on the PR (Post test-environment information)
	if j.firstRun {
		w.commentEnvironmentInformation(ctx, j, environment) // nolint: gas, errcheck
	}

	// The build finished successfully, post success state to Github
	w.updateBuildStatus( // nolint: gas, errcheck
		ctx,
		j,
		github.SuccessState,
		"Build finished",
		fmt.Sprintf("https://%s", internal.GenerateBuildURL(
			j.owner, j.repository, j.pullRequestNumber, w.options.ClusterDomain,
		)),
	)

	return nil
}

func (w *worker) deleteBuild(ctx context.Context, j *job) error {
	// Track manifest deletion time
	startTime := time.Now()
	defer w.trackTask(j, "manifest_deletion", startTime)

	w.logger.WithFields(log.Fields{
		"job_id":              j.id,
		"owner":               j.owner,
		"repository":          j.repository,
		"pull_request_number": j.pullRequestNumber,
	}).Info("deleting build")

	// Call the deleteBuildManifest method implemented in operator.go
	return w.deleteBuildManifest(ctx, j)
}
