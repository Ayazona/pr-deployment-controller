package databasetemplate

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/pr-deployment-controller/pkg/internal"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Database Template Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDatabaseTemplate{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
// nolint: gocyclo
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("databasetemplate-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DatabaseTemplate objects
	err = c.Watch(&source.Kind{Type: &testenvironmentv1alpha1.DatabaseTemplate{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to the Database objects
	return c.Watch(&source.Kind{Type: &testenvironmentv1alpha1.Database{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &testenvironmentv1alpha1.DatabaseTemplate{},
	})
}

var _ reconcile.Reconciler = &ReconcileDatabaseTemplate{}

// ReconcileDatabaseTemplate reconciles a Database Template object
type ReconcileDatabaseTemplate struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Database Template object and
// makes changes based on the state read and what is in the Database.Spec
func (r *ReconcileDatabaseTemplate) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	// Log error if reconclier returns with an error
	defer func() {
		if err != nil {
			logger := logrus.New()
			logger.WithError(err).Errorf("build reconciler error")
		}
	}()

	ctx := context.TODO()

	// Fetch the DatabaseTemplate instance
	databasetemplate := &testenvironmentv1alpha1.DatabaseTemplate{}
	err = r.Get(ctx, request.NamespacedName, databasetemplate)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Create label selector used to filter database objects
	labelSelector, err := NewDatabaseTemplateLabelSelector(request.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Retrieve existing database objects
	databases := &testenvironmentv1alpha1.DatabaseList{}
	err = r.Client.List(ctx, &client.ListOptions{Namespace: request.Namespace, LabelSelector: labelSelector}, databases)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Find out how many unclaimed databases we need to satisfy the bufferSize
	bufferSize := databasetemplate.Spec.BufferSize
	unclaimedDatabases := CountUnclaimedDatabases(databases)
	toCreate := bufferSize - unclaimedDatabases

	// Create Database objects to satisfy the bufferSize
	for toCreate > 0 {
		var name = fmt.Sprintf("%s-%s", request.Name, internal.RandomString(6))
		var database = &testenvironmentv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: request.Namespace,
				Labels: map[string]string{
					LabelDatabaseTemplate: request.Name,
				},
			},
			Spec: testenvironmentv1alpha1.DatabaseSpec{
				TemplateName: request.Name,
			},
			Status: testenvironmentv1alpha1.DatabaseStatus{
				Phase:        testenvironmentv1alpha1.DatabasePending,
				DatabaseName: databasetemplate.Spec.DatabaseName,
				Username:     databasetemplate.Spec.DatabaseUser,
				Password:     internal.RandomString(10),
				Host:         fmt.Sprintf("%s.%s", name, request.Namespace),
				Port:         5432,
			},
		}
		if err = controllerutil.SetControllerReference(databasetemplate, database, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		if err = r.Client.Create(ctx, database); err != nil {
			return reconcile.Result{}, err
		}

		toCreate--
	}

	return reconcile.Result{}, nil
}
