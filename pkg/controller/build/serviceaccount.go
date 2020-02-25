package build

import (
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (br *buildReconciler) reconcileServiceAccount() error {
	deploy := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      br.serviceAccountName,
			Namespace: br.namespace,
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &v1.ServiceAccount{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}

func (br *buildReconciler) reconcileRoleBinding() error {
	if br.options.BuildClusterRole == "" {
		return nil
	}

	deploy := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      br.serviceAccountName,
			Namespace: br.namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      br.serviceAccountName,
				Namespace: br.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     br.options.BuildClusterRole,
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &rbacv1.RoleBinding{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
