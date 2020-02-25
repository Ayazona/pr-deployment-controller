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

func (r *ReconcileDatabase) reconcilePersistentVolumeClaim(
	ctx context.Context,
	database *testenvironmentv1alpha1.Database,
	databasetemplate *testenvironmentv1alpha1.DatabaseTemplate,
	options *Options,
) error {
	deploy := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name,
			Namespace: options.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &options.StorageClassName,
			Resources:        databasetemplate.Spec.VolumeCapacity,
		},
	}
	if err := controllerutil.SetControllerReference(database, deploy, r.scheme); err != nil {
		return err
	}

	found := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Namespace: options.Namespace, Name: database.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
