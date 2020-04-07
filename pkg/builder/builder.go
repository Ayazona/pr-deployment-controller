package builder

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/kolonialno/pr-deployment-controller/pkg/docker"
	"github.com/kolonialno/pr-deployment-controller/pkg/github"
	"github.com/kolonialno/pr-deployment-controller/pkg/k8s"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// Builder defines the builder controller
type Builder interface {
	NewBuild(
		ctx context.Context, owner, repository string,
		number int64, sha, user string, firstRun, clean, force bool,
	) error
	DeleteBuild(ctx context.Context, owner, repository string, number int64) error

	Start() error
	Stop(err error)
}

type baseBuilder struct {
	logger    *log.Entry
	options   *Options
	scheduler *scheduler

	jobs    chan *job
	results chan *jobResult

	stop    chan struct{}
	stopped bool
	wg      sync.WaitGroup
}

// Options defines the options required by the builder
type Options struct {
	GitHub github.Github
	Docker docker.Docker
	K8s    *k8s.Environment

	RuntimeSummary *prometheus.SummaryVec
	ClusterDomain  string
	BuildPrefix    string
}

// New returns a new builder controller
func New(logger *log.Entry, options *Options) (Builder, error) {
	return &baseBuilder{
		logger:    logger,
		options:   options,
		scheduler: newScheduler(),

		jobs:    make(chan *job, 100),
		results: make(chan *jobResult, 100),

		stop:    make(chan struct{}, 1),
		stopped: false,
		wg:      sync.WaitGroup{},
	}, nil
}

// NewBuild creates a new build
func (b *baseBuilder) NewBuild(
	ctx context.Context,
	owner,
	repository string,
	number int64,
	ref,
	user string,
	firstRun bool,
	clean bool,
	force bool,
) error {
	if b.stopped {
		return ErrWorkerClosed
	}

	var createTime = time.Now()

	job := &job{
		id:                b.scheduler.getNextJobID(),
		owner:             owner,
		repository:        repository,
		deleteEnvironment: false,

		pullRequestNumber: number,
		ref:               ref,
		user:              user,
		firstRun:          firstRun,
		clean:             clean,
		force:             force,

		createTime: &createTime,
	}

	// Make sure the job ID is higher than the previous job for this repository
	if err := b.scheduler.scheduleJob(fmt.Sprintf("%s/%s-%d", owner, repository, number), job.id); err != nil {
		return errors.Wrap(err, "skipping job due to outdated job id")
	}

	b.jobs <- job
	b.wg.Add(1)

	return nil
}

// DeleteBuild deletes a build
func (b *baseBuilder) DeleteBuild(ctx context.Context, owner, repository string, number int64) error {
	if b.stopped {
		return ErrWorkerClosed
	}

	var createTime = time.Now()

	job := &job{
		id:                b.scheduler.getNextJobID(),
		owner:             owner,
		repository:        repository,
		deleteEnvironment: true,

		pullRequestNumber: number,

		createTime: &createTime,
	}

	// Make sure the job ID is higher than the previous job for this repository
	if err := b.scheduler.scheduleJob(fmt.Sprintf("%s/%s-%d", owner, repository, number), job.id); err != nil {
		return errors.Wrap(err, "skipping job due to outdated job id")
	}

	b.jobs <- job
	b.wg.Add(1)

	return nil
}

func (b *baseBuilder) Start() error {
	var err error

	// Start background workers
	for id := 1; id <= WorkerPoolSize; id++ {
		var w *worker

		w, err = newWorker(id, b.logger.WithField("worker_id", id), b.jobs, b.results, b.scheduler, b.options)
		if err != nil {
			return err
		}

		go w.run()
	}

	// Start result collection
	go func() {
		for {
			r, more := <-b.results
			currentTime := time.Now()

			// Track total job execution time
			b.options.RuntimeSummary.WithLabelValues(
				r.job.owner,
				r.job.repository,
				strconv.FormatInt(r.job.pullRequestNumber, 10),
				strconv.FormatInt(r.job.id, 10),
				"total",
			).Observe(currentTime.Sub(*r.job.createTime).Seconds())

			if more {
				if r.err != nil {
					b.logger.WithFields(log.Fields{
						"job_id":  r.job.id,
						"runtime": currentTime.Sub(*r.job.createTime),
						"err":     r.err,
					}).Error("job failed")
				} else {
					b.logger.WithFields(log.Fields{
						"job_id":  r.job.id,
						"runtime": currentTime.Sub(*r.job.createTime),
					}).Info("job succeeded")
				}

				b.wg.Done()
			} else {
				return
			}
		}
	}()

	// Wait on close signal and then all pending builds
	<-b.stop
	b.wg.Wait()

	// Close jobs and results channels, this stops the workers
	close(b.jobs)
	close(b.results)

	return nil
}

func (b *baseBuilder) Stop(err error) {
	if err != nil {
		b.logger.Warnf("stopping builder due to: %v", err)
	}

	b.stopped = true
	b.stop <- struct{}{}
}
