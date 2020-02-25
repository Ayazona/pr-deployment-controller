package worker

import (
	"context"
	"sync"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/test-environment-manager/pkg/k8s"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Worker defines the required interface for provisioning a database (restore data form a data dump)
// The worker manages one templateprocessor for each template stored in the api
type Worker interface {
	Start() error
	Stop(err error)
}

var (
	syncInterval = 5 * time.Second
)

type worker struct {
	logger *logrus.Entry
	k8sEnv *k8s.Environment

	namespace string

	processors    map[string]*templateprocessor
	processorLock *sync.Mutex

	wg       *sync.WaitGroup
	stopChan chan struct{}
	errChan  chan error

	dumpDownloadSeconds  *prometheus.SummaryVec
	dumpRestoreSeconds   *prometheus.SummaryVec
	databasePhaseCounter *prometheus.CounterVec
}

// New creates a new worker
func New(
	logger *logrus.Entry,
	k8sEnv *k8s.Environment,
	namespace string,
	dumpDownloadSeconds *prometheus.SummaryVec,
	dumpRestoreSeconds *prometheus.SummaryVec,
	databasePhaseCounter *prometheus.CounterVec,
) (Worker, error) {
	return &worker{
		logger: logger,
		k8sEnv: k8sEnv,

		namespace: namespace,

		processors:    make(map[string]*templateprocessor),
		processorLock: new(sync.Mutex),

		wg:       new(sync.WaitGroup),
		stopChan: make(chan struct{}, 1),
		errChan:  make(chan error, 1),

		dumpDownloadSeconds:  dumpDownloadSeconds,
		dumpRestoreSeconds:   dumpRestoreSeconds,
		databasePhaseCounter: databasePhaseCounter,
	}, nil
}

func (w *worker) Start() error {
	w.logger.Info("starting database provisoner worker")

	// Run processor sync regurlaly
	go func() {
		for range time.Tick(syncInterval) {
			err := w.syncProcessors()
			if err != nil {
				w.logger.WithError(err).Warn("sync processors failure")
			}
		}
	}()

	var err error

	// Wait for processor error or stop to be called
	select {
	case <-w.stopChan:
	case err = <-w.errChan:
	}

	// Wait on all processors to stop
	w.wg.Wait()

	return err
}

func (w *worker) Stop(err error) {
	w.logger.Info("stopping database provisoner worker")

	// Take the lock and tell all processors to stop
	w.processorLock.Lock()
	defer w.processorLock.Unlock()

	// Notify about the stopp call to the stop channel
	close(w.stopChan)

	// Stop running processors
	for _, p := range w.processors {
		p.Stop()
	}
}

// syncProcessors is responsible for making sure that the prosessors map contains a
// processor for each DatabaseTemplate stored in the apiserver.
func (w *worker) syncProcessors() error {
	ctx := context.TODO()

	// Take the lock before manipulating the processors map
	w.processorLock.Lock()
	defer w.processorLock.Unlock()

	// clusterDatabaseTemplates contains the opts for each database template inside the cluster
	var clusterDatabaseTemplates = map[string]*templateprocessorOpts{}

	// Fetch database templates
	databaseTemplates := &testenvironmentv1alpha1.DatabaseTemplateList{}
	err := w.k8sEnv.List(ctx, &client.ListOptions{Namespace: w.namespace}, databaseTemplates)
	if err != nil {
		return err
	}

	for _, databasetemplate := range databaseTemplates.Items {
		refreshInterval, err := time.ParseDuration(databasetemplate.Spec.RefreshInterval)
		if err != nil {
			w.logger.WithError(err).Errorf(
				"could not start database provisioner for template %s, could not parse refresh interval",
				databasetemplate.Name,
			)
			continue
		}

		clusterDatabaseTemplates[databasetemplate.Name] = &templateprocessorOpts{
			TemplateName:         databasetemplate.Name,
			DumpSource:           databasetemplate.Spec.DumpFile,
			DumpCredentials:      databasetemplate.Spec.Credentials,
			DumpRefreshInterval:  refreshInterval,
			DumpDownloadSeconds:  w.dumpDownloadSeconds,
			DumpRestoreSeconds:   w.dumpRestoreSeconds,
			DatabasePhaseCounter: w.databasePhaseCounter,
		}
	}

	// Make sure we have the correct proccessors configured
	// Step 1: Go through configured processors
	for name, processor := range w.processors {
		opts, ok := clusterDatabaseTemplates[name]
		if ok {
			// Update current opts
			processor.SetOpts(opts)
			// Delete for clusterDataseTemplates
			delete(clusterDatabaseTemplates, name)
		} else {
			// Stop current processor
			processor.Stop()
			// Delete from w.processors
			delete(w.processors, name)
		}
	}

	// Step 2: Add missing processors based on clusterDataseTemplates
	for name, opts := range clusterDatabaseTemplates {
		processor, err := newTemplateProcessor(w.logger, w.k8sEnv, w.namespace, opts)
		if err != nil {
			return err
		}

		// Store reference
		w.processors[name] = processor

		w.wg.Add(1)

		// Run in background and maintain waitgroup
		go func(name string) {
			w.logger.Infof("databateprovisioner for %s starting", name)
			err := processor.Start()
			w.logger.WithError(err).Infof("databaseprocessor for %s finished", name)
			if err != nil {
				w.errChan <- err
			}

			w.wg.Done()
		}(name)
	}

	return nil
}
