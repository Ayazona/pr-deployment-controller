package worker

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/pr-deployment-controller/pkg/controller/databasetemplate"
	"github.com/kolonialno/pr-deployment-controller/pkg/databaseprovisioner/fetcher"
	"github.com/kolonialno/pr-deployment-controller/pkg/databaseprovisioner/restore"
	"github.com/kolonialno/pr-deployment-controller/pkg/k8s"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// poolSize defines the number of max concurrent restores pr. database template (workers)
	poolSize = 2
)

type templateprocessor struct {
	logger *logrus.Entry
	k8sEnv *k8s.Environment
	opts   *templateprocessorOpts

	namespace string

	tasks     map[string]*restoreTask
	tasksLock *sync.RWMutex

	tempdir      string
	dumpFileLock *sync.RWMutex

	wg       *sync.WaitGroup
	stopChan chan struct{}
}

type templateprocessorOpts struct {
	TemplateName        string
	DumpSource          string
	DumpCredentials     string
	DumpRefreshInterval time.Duration

	DumpDownloadSeconds  *prometheus.SummaryVec
	DumpRestoreSeconds   *prometheus.SummaryVec
	DatabasePhaseCounter *prometheus.CounterVec
}

func newTemplateProcessor(
	logger *logrus.Entry,
	k8sEnv *k8s.Environment,
	namespace string,
	opts *templateprocessorOpts,
) (*templateprocessor, error) {
	processor := &templateprocessor{
		logger: logger,
		k8sEnv: k8sEnv,

		namespace: namespace,

		tasks:     make(map[string]*restoreTask),
		tasksLock: new(sync.RWMutex),

		dumpFileLock: new(sync.RWMutex),

		wg:       new(sync.WaitGroup),
		stopChan: make(chan struct{}, 1),
	}

	processor.SetOpts(opts)

	err := processor.createTempDir()
	if err != nil {
		return nil, err
	}

	return processor, nil
}

func (t *templateprocessor) SetOpts(opts *templateprocessorOpts) {
	t.opts = opts
}

func (t *templateprocessor) createTempDir() error {
	dir, err := ioutil.TempDir("", "pr-deployment-controller")
	if err != nil {
		return err
	}

	t.tempdir = dir

	return nil
}

func (t *templateprocessor) dumpfileLocation() string {
	return path.Join(t.tempdir, "dumpfile")
}

func (t *templateprocessor) Start() error {
	// Create context with cancel hook, used to stop long-running command execution
	ctx, cancel := context.WithCancel(context.TODO())

	// Cancel context when the the stopChan closes
	go func() {
		<-t.stopChan
		cancel()
	}()

	// Start database watcher
	dw, err := newDatabaseWatcher(t.opts, t.logger, t.k8sEnv, t.namespace, &t.tasks, t.tasksLock)
	if err != nil {
		return err
	}
	go func() {
		watcherErr := dw.start(ctx)
		t.logger.WithError(watcherErr).Warn("database watcher stopped")
	}()

	// Start dumpfile fetcher
	dff, err := newDumpFileFetcher(t.opts, t.logger, t.dumpFileLock, t.dumpfileLocation(), t.opts.DumpDownloadSeconds)
	if err != nil {
		return err
	}
	go func() {
		err := dff.start(ctx)
		t.logger.WithError(err).Warn("dumpfile fetcher stopped")
	}()

	// Create restore workers
	for id := 0; id < poolSize; id++ {
		worker, err := newRestoreWorker(
			t.logger,
			t.k8sEnv,
			t.namespace,
			&t.tasks,
			t.tasksLock,
			t.dumpfileLocation(),
			t.dumpFileLock,
			t.opts.DumpRestoreSeconds,
		)
		if err != nil {
			return err
		}

		t.wg.Add(1)

		go func() {
			worker.start(ctx) // nolint: errcheck
			t.wg.Done()
		}()
	}

	// Wait on restore workers to close
	t.wg.Wait()

	// Cleanup tempdir used to store dump files
	if t.tempdir != "" {
		defer os.RemoveAll(t.tempdir) // nolint: errcheck
	}

	return nil
}

func (t *templateprocessor) Stop() {
	close(t.stopChan)
}

/*
* Database watcher
**/

type databaseWatcher struct {
	templateopts *templateprocessorOpts
	logger       *logrus.Entry
	k8sEnv       *k8s.Environment

	namespace string

	tasks *map[string]*restoreTask
	lock  *sync.RWMutex
}

func newDatabaseWatcher(
	templateopts *templateprocessorOpts,
	logger *logrus.Entry,
	k8sEnv *k8s.Environment,
	namespace string,
	tasks *map[string]*restoreTask,
	lock *sync.RWMutex,
) (*databaseWatcher, error) {
	return &databaseWatcher{
		templateopts: templateopts,
		logger:       logger,
		k8sEnv:       k8sEnv,

		namespace: namespace,

		tasks: tasks,
		lock:  lock,
	}, nil
}

func (dw *databaseWatcher) start(ctx context.Context) error {
	dw.logger.Info("starting database watcher")
	defer dw.logger.Info("stopped database watcher")

	ticker := time.NewTicker(syncInterval)

	for range ticker.C {
		labelSelector, err := databasetemplate.NewDatabaseTemplateLabelSelector(dw.templateopts.TemplateName)
		if err != nil {
			dw.logger.WithError(err).Warn("could not initialize label selector")
			time.Sleep(10 * time.Second)
			continue
		}

		databases := &testenvironmentv1alpha1.DatabaseList{}
		err = dw.k8sEnv.List(ctx, &client.ListOptions{Namespace: dw.namespace, LabelSelector: labelSelector}, databases)
		if err != nil {
			dw.logger.WithError(err).Warnf("fetch databases %s failed", dw.templateopts.TemplateName)
			time.Sleep(10 * time.Second)
			continue
		}

		for _, database := range databases.Items {
			database := database
			if database.Status.Phase == testenvironmentv1alpha1.DatabasePending {
				err = dw.restoreInstance(ctx, &database)
				if err != nil {
					dw.logger.WithError(err).Warnf("could not create restore task for %s", database.Name)
				}
			} else if database.Status.Phase == testenvironmentv1alpha1.DatabaseProvisioning {
				err = dw.checkRestoreProcess(ctx, &database)
				if err != nil {
					dw.logger.WithError(err).Warnf("could not verify %s provision process", database.Name)
				}
			}
		}
	}

	return nil
}

// Add database to the restore task queue if not already exists
func (dw *databaseWatcher) restoreInstance(ctx context.Context, database *testenvironmentv1alpha1.Database) error {
	dw.lock.Lock()
	defer dw.lock.Unlock()

	_, ok := (*dw.tasks)[database.Name]
	if !ok {
		dw.logger.Infof("creating restore task for %s", database.Name)
		(*dw.tasks)[database.Name] = &restoreTask{
			name:         database.Name,
			databasename: database.Status.DatabaseName,
			host:         database.Status.Host,
			port:         database.Status.Port,
			username:     database.Status.Username,
			password:     database.Status.Password,
		}
	}

	return nil
}

// Make sure the restore process is performed by us if the database is in provision phase
func (dw *databaseWatcher) checkRestoreProcess(ctx context.Context, database *testenvironmentv1alpha1.Database) error {
	dw.lock.RLock()
	defer dw.lock.RUnlock()

	_, ok := (*dw.tasks)[database.Name]
	if !ok {
		dw.logger.Warnf("restore process for %s not owned by us, deleting database", database.Name)

		err := dw.k8sEnv.Delete(ctx, database)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

/*
* Dump file fetcher
**/

type dumpFileFetcher struct {
	templateopts *templateprocessorOpts
	logger       *logrus.Entry
	lock         *sync.RWMutex
	destination  string

	dumpDownloadSeconds *prometheus.SummaryVec
}

func newDumpFileFetcher(
	templateopts *templateprocessorOpts,
	logger *logrus.Entry,
	lock *sync.RWMutex,
	destibation string,
	dumpDownloadSeconds *prometheus.SummaryVec,
) (*dumpFileFetcher, error) {
	return &dumpFileFetcher{
		templateopts: templateopts,
		logger:       logger,
		lock:         lock,
		destination:  destibation,

		dumpDownloadSeconds: dumpDownloadSeconds,
	}, nil
}

func (dff *dumpFileFetcher) start(ctx context.Context) error {
	fetchDumpFile := func(source, credentials string) error {
		f, err := fetcher.New(dff.logger, &fetcher.FetchOpts{
			DatabaseTemplate:    dff.templateopts.TemplateName,
			Source:              source,
			Destibantion:        dff.destination,
			Credentials:         credentials,
			DumpDownloadSeconds: dff.dumpDownloadSeconds,
		})
		if err != nil {
			return err
		}

		var doneChan = make(chan struct{}, 1)

		go func() {
			select {
			case <-ctx.Done():
				f.Stop()
			case <-doneChan:
			}
		}()

		dff.logger.Infof("fetching dump file %s", source)
		defer func() {
			if err != nil {
				dff.logger.WithError(err).Warnf("fetching dump file %s stopped", source)
			} else {
				dff.logger.Infof("fetching dump file %s done", source)
			}
		}()

		dff.lock.Lock()
		defer dff.lock.Unlock()

		err = f.Start()
		close(doneChan)

		return err
	}

	for {
		fetchDumpFile(dff.templateopts.DumpSource, dff.templateopts.DumpCredentials) // nolint: errcheck, gas
		<-time.After(dff.templateopts.DumpRefreshInterval)
	}
}

/*
* Restore worker
**/

type restoreTask struct {
	name         string
	databasename string
	host         string
	port         int64
	username     string
	password     string

	running bool
}

type restoreWorker struct {
	logger *logrus.Entry
	k8sEnv *k8s.Environment

	namespace string

	tasks              *map[string]*restoreTask
	tasksLock          *sync.RWMutex
	dumpfile           string
	fileLock           *sync.RWMutex
	dumpRestoreSeconds *prometheus.SummaryVec
}

func newRestoreWorker(
	logger *logrus.Entry,
	k8sEnv *k8s.Environment,
	namespace string,
	tasks *map[string]*restoreTask,
	tasksLock *sync.RWMutex,
	dumpfile string,
	fileLock *sync.RWMutex,
	dumpRestoreSeconds *prometheus.SummaryVec,
) (*restoreWorker, error) {
	worker := &restoreWorker{
		logger:             logger,
		k8sEnv:             k8sEnv,
		namespace:          namespace,
		tasks:              tasks,
		tasksLock:          tasksLock,
		dumpfile:           dumpfile,
		fileLock:           fileLock,
		dumpRestoreSeconds: dumpRestoreSeconds,
	}

	return worker, nil
}

func (rw *restoreWorker) start(ctx context.Context) error {
	rw.logger.Info("starting restore worker")
	defer rw.logger.Info("stopped restore worker")

out:
	for {
		select {
		case <-ctx.Done():
			break out
		default:
			var currenttask *restoreTask

			rw.tasksLock.Lock()
			for _, task := range *rw.tasks {
				if !task.running {
					task.running = true
					currenttask = task
					break
				}
			}
			rw.tasksLock.Unlock()

			if currenttask != nil {
				rechedule, err := rw.processTask(ctx, currenttask)
				if err != nil {
					rw.logger.WithError(err).Warnf("database restore for database %s returned with error", currenttask.name)
				}

				// finish task or put it back on the queue
				rw.tasksLock.Lock()
				if !rechedule {
					rw.logger.Infof("restore of %s database removed from queue", currenttask.name)
					delete(*rw.tasks, currenttask.name)
				} else {
					rw.logger.Infof("restore of %s database put back on queue", currenttask.name)
					currenttask.running = false
				}
				rw.tasksLock.Unlock()
			} else {
				time.Sleep(time.Second)
			}
		}
	}

	return nil
}

func (rw *restoreWorker) dumpFileExists() (bool, error) {
	if _, err := os.Stat(rw.dumpfile); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (rw *restoreWorker) processTask(ctx context.Context, task *restoreTask) (bool, error) {
	// Take the file read lock to make sure this isnt changed in the background
	rw.fileLock.RLock()
	defer rw.fileLock.RUnlock()

	// Make sure the dump file exists in disk
	exist, err := rw.dumpFileExists()
	if err != nil || !exist {
		return true, err
	}

	rw.logger.WithField("database", task.name).Info("retrieving database object")

	// Lookup database object
	database, err := rw.getDatabase(ctx, task.name)
	if err != nil {
		// Dont rechedule task, let the database watcher create the task again
		return false, err
	}

	rw.logger.WithField("database", task.name).Info("setting database in provisioning mode")

	// Set the database in provisioning mode
	err = rw.setDatabasePhase(ctx, database, testenvironmentv1alpha1.DatabaseProvisioning)
	if err != nil {
		// Dont reschedule task, let the database watcher create the task again
		return false, err
	}

	rw.logger.WithField("database", task.name).Info("restoring data")

	// Create restore instance
	r, err := restore.NewPostgresRestore(rw.logger, &restore.PostgresRestoreOpts{
		Name:     task.name,
		Hostname: task.host,
		Port:     task.port,
		Database: task.databasename,
		Username: task.username,
		Password: task.password,
		DumpFile: rw.dumpfile,
	}, rw.dumpRestoreSeconds)
	if err != nil {
		// Dont reshedule, let the database watcher cleanup the database
		return false, err
	}

	// Define the restore function
	performRestore := func() error {
		var doneChan = make(chan struct{}, 1)

		go func() {
			select {
			case <-ctx.Done():
				r.Stop()
			case <-doneChan:
			}
		}()

		err = r.Start()
		close(doneChan)

		return err
	}

	if err = performRestore(); err != nil {
		// Dont reshedule, let the database watcher cleanup the database
		return false, err
	}

	rw.logger.WithField("database", task.name).Info("setting database in ready mode")

	// Set the database in the ready phase
	err = rw.setDatabasePhase(ctx, database, testenvironmentv1alpha1.DatabaseReady)
	if err != nil {
		return false, err
	}

	rw.logger.WithField("database", task.name).Info("restore done")

	return false, nil
}

func (rw *restoreWorker) getDatabase(ctx context.Context, name string) (*testenvironmentv1alpha1.Database, error) {
	database := &testenvironmentv1alpha1.Database{}
	err := rw.k8sEnv.Get(ctx, types.NamespacedName{Name: name, Namespace: rw.namespace}, database)

	return database, err
}

func (rw *restoreWorker) setDatabasePhase(
	ctx context.Context,
	database *testenvironmentv1alpha1.Database,
	phase testenvironmentv1alpha1.DatabasePhase,
) error {
	database.Status.Phase = phase

	return rw.k8sEnv.Update(ctx, database)
}
