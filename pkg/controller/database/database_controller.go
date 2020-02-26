package database

import (
	"context"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// Options gives the database access to values from the main application
type Options struct {
	Logger             *logrus.Entry
	Namespace          string
	ServiceAccountName string
	StorageClassName   string
}

var options *Options

// SetOptions sets the database controller options
// This must be called before the initialization of the controller
func SetOptions(o *Options) {
	options = o
}

// Add creates a new Database Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	if options == nil {
		return ErrOptionsNotConfigured
	}

	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDatabase{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
// nolint: gocyclo
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("database-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Database
	err = c.Watch(&source.Kind{Type: &testenvironmentv1alpha1.Database{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to the Deployment object
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Database{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to the Service object
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Database{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to the Persistent volume claim object
	return c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.Database{},
	})
}

var _ reconcile.Reconciler = &ReconcileDatabase{}

// ReconcileDatabase reconciles a Database object
type ReconcileDatabase struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Database object and makes changes based on the state read
// and what is in the Database.Spec
func (r *ReconcileDatabase) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	// Log error if reconclier returns with an error
	defer func() {
		if err != nil {
			logger := logrus.New()
			logger.WithError(err).Errorf("build reconciler error")
		}
	}()

	ctx := context.TODO()

	// Fetch the Database instance
	database := &testenvironmentv1alpha1.Database{}
	err = r.Get(ctx, request.NamespacedName, database)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the DatabaseTemplate instance
	databasetemplate := &testenvironmentv1alpha1.DatabaseTemplate{}
	err = r.Get(
		ctx,
		types.NamespacedName{Name: database.Spec.TemplateName, Namespace: request.Namespace},
		databasetemplate,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Reconcile persistent volume claim
	err = r.reconcilePersistentVolumeClaim(ctx, database, databasetemplate, options)
	if err != nil {
		return reconcile.Result{}, nil
	}

	// Reconcile deployment
	err = r.reconcileDeployment(ctx, database, databasetemplate, options)
	if err != nil {
		return reconcile.Result{}, nil
	}

	// Reconcile service
	err = r.reconcileService(ctx, database, options)
	if err != nil {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}
