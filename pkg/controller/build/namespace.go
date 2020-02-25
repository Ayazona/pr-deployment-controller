package build

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (br *buildReconciler) reconcileNamespace() error {
	deploy := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: br.namespace,
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &v1.Namespace{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name}, found)
	if err != nil && errors.IsNotFound(err) {
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
