package cmd

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kolonialno/pr-deployment-controller/cmd/internal"
	"github.com/kolonialno/pr-deployment-controller/pkg"
	"github.com/kolonialno/pr-deployment-controller/pkg/apis"
	"github.com/kolonialno/pr-deployment-controller/pkg/builder"
	"github.com/kolonialno/pr-deployment-controller/pkg/cleanup"
	"github.com/kolonialno/pr-deployment-controller/pkg/controller"
	"github.com/kolonialno/pr-deployment-controller/pkg/controller/build"
	"github.com/kolonialno/pr-deployment-controller/pkg/controller/database"
	"github.com/kolonialno/pr-deployment-controller/pkg/databaseprovisioner/worker"
	"github.com/kolonialno/pr-deployment-controller/pkg/debug"
	"github.com/kolonialno/pr-deployment-controller/pkg/docker"
	"github.com/kolonialno/pr-deployment-controller/pkg/github"
	"github.com/kolonialno/pr-deployment-controller/pkg/k8s"
	"github.com/kolonialno/pr-deployment-controller/pkg/status"
	"github.com/kolonialno/pr-deployment-controller/pkg/webhook"
	"github.com/oklog/oklog/pkg/group"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	internal.StringFlag(runCmd, "statusAddr", "HTTP status server listen address", ":8000")
	internal.StringFlag(runCmd, "listenAddr", "HTTP server listen address", ":9000")
	internal.StringFlag(runCmd, "debugAddr", "HTTP debug server listen address", ":9001")
	internal.BoolFlag(runCmd, "jsonLogging", "log output as json", true)

	internal.StringFlag(runCmd, "namespace", "namespace this operator runs in", "")
	internal.StringFlag(runCmd, "clusterDomain", "wildcard domain pointed to the cluster", "")
	internal.StringFlag(runCmd, "databaseNamespace", "namespace databases should run in", "")

	internal.StringFlag(runCmd, "buildClusterRole", "Bind build service account to this cluster role", "")

	internal.StringFlag(runCmd, "statusServiceName", "the name of the service exposing the status server", "")
	internal.Int64Flag(runCmd, "statusServicePort", "the service port exposing the status server", 8000)

	internal.StringFlag(runCmd, "dockerHost", "Docker daemon listen address", "")
	internal.StringFlag(runCmd, "dockerAPIVersion", "Docker API version", "1.39")
	internal.StringFlag(runCmd, "dockerCertFile", "Docker certificate path", "")
	internal.StringFlag(runCmd, "dockerKeyFile", "Docker certificate key path", "")
	internal.StringFlag(runCmd, "dockerCAFile", "Docker CA certificate path", "")
	internal.StringFlag(runCmd, "dockerRegistry", "Docker registry prefix", "")
	internal.StringFlag(runCmd, "dockerRegistryUsername", "Docker registry username", "")
	internal.StringFlag(runCmd, "dockerRegistryPassword", "Docker registry password", "")
	internal.StringFlag(runCmd, "dockerRegistryPasswordFile", "Docker registry password file", "")

	internal.StringFlag(runCmd, "githubWebhookSecret", "Secret used to sign GitHub webhooks", "")
	internal.StringFlag(runCmd, "githubAccessToken", "Access token used to authenticate with the GitHub API", "")
	internal.StringFlag(runCmd, "githubUsername", "GitHub token owner username, used to filter comments", "")

	internal.StringFlag(
		runCmd,
		"databaseStorageClassName",
		"Storage class used to provision persistent storage for databases",
		"",
	)
	internal.StringFlag(runCmd, "databaseServiceAccountName", "Service account applied to databases", "")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the operator",
	Long:  "Starts the reconciliation loop that maintains the test environments.",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return internal.CheckFlags(
			internal.RequireString("statusAddr"),
			internal.RequireString("listenAddr"),
			internal.RequireString("debugAddr"),

			internal.RequireString("namespace"),
			internal.RequireString("clusterDomain"),
			internal.RequireString("databaseNamespace"),

			internal.RequireString("statusServiceName"),

			internal.RequireString("dockerHost"),
			internal.RequireString("dockerAPIVersion"),
			internal.RequireString("dockerCertFile"),
			internal.RequireString("dockerKeyFile"),
			internal.RequireString("dockerCAFile"),

			internal.RequireString("githubWebhookSecret"),
			internal.RequireString("githubAccessToken"),

			internal.RequireString("databaseStorageClassName"),
			internal.RequireString("databaseServiceAccountName"),
		)
	},
	RunE: func(_ *cobra.Command, args []string) error {

		var statusAddr, listenAddr, debugAddr string
		var jsonLogging bool
		var namespace, clusterDomain, databaseNamespace string
		var buildClusterRole string
		var statusServiceName string
		var statusServicePort int64
		var dockerHost, dockerAPIVersion, dockerCertFile, dockerKeyFile, dockerCAFile string
		var dockerRegistry, dockerRegistryUsername, dockerRegistryPassword, dockerRegistryPasswordFile string
		var githubWebhookSecret, githubAccessToken, githubUsername string
		var databaseStorageClassName, databaseServiceAccountName string
		{
			statusAddr = viper.GetString("statusAddr")
			listenAddr = viper.GetString("listenAddr")
			debugAddr = viper.GetString("debugAddr")
			jsonLogging = viper.GetBool("jsonLogging")

			namespace = viper.GetString("namespace")
			clusterDomain = viper.GetString("clusterDomain")
			databaseNamespace = viper.GetString("databaseNamespace")

			buildClusterRole = viper.GetString("buildClusterRole")

			statusServiceName = viper.GetString("statusServiceName")
			statusServicePort = viper.GetInt64("statusServicePort")

			dockerHost = viper.GetString("dockerHost")
			dockerAPIVersion = viper.GetString("dockerAPIVersion")
			dockerCertFile = viper.GetString("dockerCertFile")
			dockerKeyFile = viper.GetString("dockerKeyFile")
			dockerCAFile = viper.GetString("dockerCAFile")
			dockerRegistry = viper.GetString("dockerRegistry")
			dockerRegistryUsername = viper.GetString("dockerRegistryUsername")
			dockerRegistryPassword = viper.GetString("dockerRegistryPassword")
			dockerRegistryPasswordFile = viper.GetString("dockerRegistryPasswordFile")

			githubWebhookSecret = viper.GetString("githubWebhookSecret")
			githubAccessToken = viper.GetString("githubAccessToken")
			githubUsername = viper.GetString("githubUsername")

			databaseStorageClassName = viper.GetString("databaseStorageClassName")
			databaseServiceAccountName = viper.GetString("databaseServiceAccountName")
		}

		// Static value prefix used to name build namespaces
		buildPrefix := "test-environment-"

		// Initialize metrics
		jobDurationSeconds := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "test_environment",
			Subsystem: "builder",
			Name:      "job_duration_seconds",
			Help:      "Request duration in seconds.",
		}, []string{"owner", "repository", "pull_request", "job", "operation"})
		prometheus.Register(jobDurationSeconds) // nolint: errcheck, gas

		dumpDownloadSeconds := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "test_environment",
			Subsystem: "databaseprovisioner",
			Name:      "dump_download_seconds",
			Help:      "Database dump dowload time in seconds",
		}, []string{"database"})
		prometheus.Register(dumpDownloadSeconds) // nolint: errcheck

		dumpRestoreSeconds := prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "test_environment",
			Subsystem: "databaseprovisioner",
			Name:      "dump_restore_seconds",
			Help:      "Dump restore time in seconds",
		}, []string{"database"})
		prometheus.Register(dumpRestoreSeconds) // nolint: errcheck

		databasePhaseCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "test_environment",
			Subsystem: "databaseprovisioner",
			Name:      "database_phases",
			Help:      "Database count per phases",
		}, []string{"database", "phase"})
		prometheus.Register(databasePhaseCounter) // nolint: errcheck

		// Log output as json
		if jsonLogging {
			log.SetFormatter(&log.JSONFormatter{})
		}

		// Initialize logger
		logger := log.WithFields(log.Fields{
			"app": pkg.App.Name,
		})

		// Setup GitHub interface
		githubController, err := github.New(logger.WithField("component", "github"), githubAccessToken)
		if err != nil {
			return err
		}

		// Setup apiserver client config (Load in-cluster kubeconfig)
		cfg, err := config.GetConfig()
		if err != nil {
			return errors.Wrap(err, "unable to set up client config")
		}

		// Setup controller manager
		var syncPeriod = 1 * time.Hour
		mgr, err := manager.New(cfg, manager.Options{
			LeaderElection:          true,
			LeaderElectionNamespace: namespace,
			LeaderElectionID:        "pr-deployment-controller-lock",
			SyncPeriod:              &syncPeriod,
		})
		if err != nil {
			return errors.Wrap(err, "unable to set up the controller manager")
		}

		// Enable custom api resource types
		if err = apis.AddToScheme(mgr.GetScheme()); err != nil {
			return errors.Wrap(err, "unable add APIs to scheme")
		}

		// Add custom runnable to detect lock acquisition
		lockacquisition := internal.NewRunnable()
		if err = mgr.Add(lockacquisition); err != nil {
			return errors.Wrap(err, "cannot add lock acquisition runnable")
		}

		// Setup k8s environment, used outside the controller manager.
		k8sEnv, err := k8s.New(mgr, namespace, buildPrefix)
		if err != nil {
			return err
		}

		// Set build controller options
		build.SetOptions(&build.Options{
			Logger:            logger.WithField("component", "build-controller"),
			Namespace:         namespace,
			BuildPrefix:       buildPrefix,
			ClusterDomain:     clusterDomain,
			GitHub:            githubController,
			IstioNamespace:    "istio-system",
			IstioGateway:      "default",
			BuildClusterRole:  buildClusterRole,
			StatusServiceName: statusServiceName,
			StatusServicePort: statusServicePort,
		})

		// Set database controller options
		database.SetOptions(&database.Options{
			Logger:             logger.WithField("component", "database-controller"),
			Namespace:          namespace,
			ServiceAccountName: databaseServiceAccountName,
			StorageClassName:   databaseStorageClassName,
		})

		// Add controllers to the manager
		if err = controller.AddToManager(mgr); err != nil {
			return errors.Wrap(err, "unable to register controllers to the manager")
		}

		// Setup docker interface
		dockerController, err := docker.New(logger.WithField("component", "docker"), &docker.Config{
			Host:                 dockerHost,
			APIVersion:           dockerAPIVersion,
			CertFile:             dockerCertFile,
			KeyFile:              dockerKeyFile,
			CACertFile:           dockerCAFile,
			Registry:             dockerRegistry,
			RegistryUsername:     dockerRegistryUsername,
			RegistryPassword:     dockerRegistryPassword,
			RegistryPasswordFile: dockerRegistryPasswordFile,
		})
		if err != nil {
			return err
		}

		// Setup a new builder
		builderController, err := builder.New(logger.WithField("component", "builder"), &builder.Options{
			GitHub: githubController,
			Docker: dockerController,
			K8s:    k8sEnv,

			RuntimeSummary: jobDurationSeconds,
			ClusterDomain:  clusterDomain,
			BuildPrefix:    buildPrefix,
		})
		if err != nil {
			return errors.Wrap(err, "could not create the build controller (the operator instance)")
		}

		// Setup the background cleanup task
		cleanupWorker, err := cleanup.New(logger.WithField("component", "cleanup"), k8sEnv, githubController)
		if err != nil {
			return errors.Wrap(err, "could not create the cleanup worker")
		}

		// Setup webhook http handlers
		webhookHandler, err := webhook.New(
			logger.WithField("component", "webhook"),
			builderController,
			githubController,
			githubWebhookSecret,
			githubUsername,
		)
		if err != nil {
			return err
		}

		// Setup the status http handlers
		statusHandler, err := status.New(logger.WithField("component", "status"), cfg, k8sEnv)
		if err != nil {
			return err
		}

		// Setup databaseprovisioner worker
		databaseprovisioner, err := worker.New(
			logger.WithField("component", "databaseprovisioner"),
			k8sEnv,
			databaseNamespace,
			dumpDownloadSeconds,
			dumpRestoreSeconds,
			databasePhaseCounter,
		)
		if err != nil {
			return err
		}

		// Setup debugserver http handers
		debugHandler, err := debug.New()
		if err != nil {
			return err
		}

		//
		// Run each component as a separate goroutine (With the oklog/group package)
		//

		var g group.Group
		{
			// Reconciliation loop
			ctx, cancel := context.WithCancel(context.Background())
			g.Add(func() error {
				return mgr.Start(ctx.Done())
			}, func(err error) {
				cancel()
			})
		}
		{
			// HTTP webhook server
			publicListener, err := net.Listen("tcp", listenAddr)
			if err != nil {
				return errors.Wrap(err, "could not create listener")
			}

			g.Add(func() error {
				server := http.Server{
					Handler:      webhookHandler,
					WriteTimeout: 30 * time.Second,
					ReadTimeout:  30 * time.Second,
				}

				// Wait on mgr lock acquisition
				logger.Info("waiting on mgr lock acquisition")
				<-lockacquisition.Done()
				logger.Info("mgr lock acquired, starting webhook server")

				return server.Serve(publicListener)
			}, func(err error) {
				lockacquisition.Close()
				publicListener.Close() // nolint: errcheck, gas
			})
		}
		{
			// HTTP status server
			statusListener, err := net.Listen("tcp", statusAddr)
			if err != nil {
				return errors.Wrap(err, "could not create listener")
			}

			g.Add(func() error {
				server := http.Server{
					Handler:      statusHandler,
					WriteTimeout: 10 * time.Second,
					ReadTimeout:  10 * time.Second,
				}

				return server.Serve(statusListener)
			}, func(err error) {
				statusListener.Close() // nolint: errcheck, gas
			})
		}
		{
			// HTTP debug server
			debugListener, err := net.Listen("tcp", debugAddr)
			if err != nil {
				return errors.Wrap(err, "could not create listener")
			}

			g.Add(func() error {
				server := http.Server{
					Handler:      debugHandler,
					WriteTimeout: 10 * time.Second,
					ReadTimeout:  10 * time.Second,
				}
				return server.Serve(debugListener)
			}, func(err error) {
				debugListener.Close() // nolint: errcheck, gas
			})
		}
		{
			// Builder worker
			g.Add(builderController.Start, builderController.Stop)
		}
		{
			// Cleanup worker
			g.Add(cleanupWorker.Runnable())
		}
		{
			// Database provisioner
			g.Add(func() error {

				// Wait on mgr lock acquisition
				logger.Info("waiting on mgr lock acquisition")
				<-lockacquisition.Done()
				logger.Info("mgr lock acquired, starting webhook server")

				return databaseprovisioner.Start()

			}, func(err error) {
				lockacquisition.Close()
				databaseprovisioner.Stop(err)
			})
		}
		{
			// Listen on interrupts
			cancelInterrupt := make(chan struct{})
			g.Add(func() error {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
				select {
				case sig := <-c:
					logger.Errorf("received signal %s", sig)
					return nil
				case <-cancelInterrupt:
					return nil
				}
			}, func(error) {
				close(cancelInterrupt)
			})
		}

		return g.Run()

	},
}
