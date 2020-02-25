package database

import (
	"context"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileDatabase) reconcileService(
	ctx context.Context,
	database *testenvironmentv1alpha1.Database,
	options *Options,
) error {
	deploy := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name,
			Namespace: options.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: Labels(database),
			Ports: []corev1.ServicePort{{
				Name:     "tcp-postgres",
				Protocol: corev1.ProtocolTCP,
				Port:     5432,
			}},
		},
	}
	if err := controllerutil.SetControllerReference(database, deploy, r.scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Namespace: options.Namespace, Name: database.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
