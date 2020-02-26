package build

import (
	"context"
	"fmt"
	"time"

	"github.com/kolonialno/pr-deployment-controller/pkg/apis/networking/v1alpha3"
	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/pr-deployment-controller/pkg/github"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Options gives the build access to values from the main application
type Options struct {
	Logger            *logrus.Entry
	Namespace         string
	BuildPrefix       string
	ClusterDomain     string
	GitHub            github.Github
	IstioNamespace    string
	IstioGateway      string
	BuildClusterRole  string
	StatusServiceName string
	StatusServicePort int64
}

var options *Options

// SetOptions sets the build controller options
// This must be called before the initialization of the controller
func SetOptions(o *Options) {
	options = o
}

// buildReconciler contains the shared props for a reconciliation run and the associated methods
type buildReconciler struct {
	ctx     context.Context
	r       *ReconcileBuild
	options *Options

	build              *testenvironmentv1alpha1.Build
	environment        *testenvironmentv1alpha1.Environment
	namespace          string
	serviceAccountName string

	logger *log.Entry
}

// Add creates a new Build Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	if options == nil {
		return ErrOptionsNotConfigured
	}

	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBuild{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
// nolint: gocyclo
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("build-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Build
	err = c.Watch(&source.Kind{Type: &testenvironmentv1alpha1.Build{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to Namespaces
	err = c.Watch(&source.Kind{Type: &v1.Namespace{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to ServiceAccounts
	err = c.Watch(&source.Kind{Type: &v1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to ClusterRoleBindings
	err = c.Watch(&source.Kind{Type: &rbacv1.RoleBinding{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to ConfigMaps
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to Deployments
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to Services
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
	if err != nil {
		return err
	}

	// Advanced watch - no resource owner defiend
	mapFn := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		result := []reconcile.Request{}

		if app, ok := a.Meta.GetLabels()["app"]; ok {
			result = append(
				result,
				reconcile.Request{
					NamespacedName: types.NamespacedName{Name: app, Namespace: options.Namespace},
				},
			)
		}

		return result
	})

	// Watch endpoints
	err = c.Watch(&source.Kind{Type: &corev1.Endpoints{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: mapFn,
	})
	if err != nil {
		return err
	}

	// Watch pods
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: mapFn,
	})
	if err != nil {
		return err
	}

	// Watch for changes to VirtualServcies
	return c.Watch(&source.Kind{Type: &v1alpha3.VirtualService{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Build{},
	})
}

var _ reconcile.Reconciler = &ReconcileBuild{}

// ReconcileBuild reconciles a Build object
type ReconcileBuild struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Build object and makes changes based on the state read
// and what is in the Build.Spec
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=testenvironment.kolonial.no,resources=builds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=testenvironment.kolonial.no,resources=builds/status,verbs=get;update;patch
// nolint: gocyclo
func (r *ReconcileBuild) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	// Log error if reconclier returns with an error
	defer func() {
		if err != nil {
			options.Logger.WithError(err).Errorf("build reconciler error")
		}
	}()

	ctx := context.Background()

	// Fetch the Build instance
	build := &testenvironmentv1alpha1.Build{}
	err = r.Get(context.TODO(), request.NamespacedName, build)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Environment instance
	environment := &testenvironmentv1alpha1.Environment{}
	if err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: request.NamespacedName.Namespace,
		Name:      build.Spec.Environment,
	}, environment); err != nil {
		return reconcile.Result{}, err
	}

	// Generate values used during the reconciliation
	namespace := fmt.Sprintf(
		"%s%s-%s-%d",
		options.BuildPrefix,
		build.Spec.Git.Owner,
		build.Spec.Git.Repository,
		build.Spec.Git.PullRequestNumber,
	)
	serviceAccountName := "test-environment"

	logger := options.Logger.WithField("namespace", namespace)

	br := buildReconciler{
		ctx:     ctx,
		r:       r,
		options: options,

		build:              build,
		environment:        environment,
		namespace:          namespace,
		serviceAccountName: serviceAccountName,

		logger: logger,
	}

	// Create build namespace
	err = br.reconcileNamespace()
	if err != nil {
		logger.WithError(err).Error("could not find environment for build")
		return reconcile.Result{}, err
	}

	// Create service account
	err = br.reconcileServiceAccount()
	if err != nil {
		logger.WithError(err).Error("could not reconcile serviceaccount")
		return reconcile.Result{}, err
	}

	// Create service account role binding
	err = br.reconcileRoleBinding()
	if err != nil {
		logger.WithError(err).Error("could not reconcile role binding")
		return reconcile.Result{}, err
	}

	// Check if the build image has changed
	imageChanged, err := br.hasImageChanged()
	if err != nil {
		logger.WithError(err).Error("could not lookup image changes")
		return reconcile.Result{}, err
	}

	// Create services
	err = br.reconcileServices(imageChanged)
	if err != nil {
		logger.WithError(err).Error("could not reconcile services")
		return reconcile.Result{}, err
	}

	// Shared env variables
	err = br.reconcileSharedEnv()
	if err != nil {
		logger.WithError(err).Error("could not reconcile sharedEnv")
		// Wait 1 minute before trying again, a database may need to be provisioned first
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// Create tasks
	err = br.reconcileTasks(imageChanged)
	if err != nil {
		logger.WithError(err).Error("could not reconcile tasks")
		return reconcile.Result{}, err
	}

	// Create containers
	err = br.reconcileContainers(imageChanged)
	if err != nil {
		logger.WithError(err).Error("could not reconcile containers")
		return reconcile.Result{}, err
	}

	// Create routing rules
	err = br.reconcileViritualServices()
	if err != nil {
		logger.WithError(err).Error("could not reconcile routing rules")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
