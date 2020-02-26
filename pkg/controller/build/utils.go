package build

import (
	"fmt"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (br *buildReconciler) hasImageChanged() (bool, error) {
	/**
	* Try to find a image used in the cluster, return true if the evironment image
	* is different from the image used in the cluster.
	**/

	br.logger.Info("checking for image changes")

	for _, container := range br.environment.Spec.Containers {
		deploymentName := fmt.Sprintf("%s-container", container.Name)

		found := &appsv1.Deployment{}
		if err := br.r.Get(
			br.ctx, types.NamespacedName{Name: deploymentName, Namespace: br.namespace}, found,
		); err == nil {
			imageChanged := found.Spec.Template.Spec.Containers[0].Image != br.build.Spec.Image
			br.logger.WithField("changed", imageChanged).Info("image detected")
			return imageChanged, nil
		} else if !errors.IsNotFound(err) {
			return false, err
		}
	}

	br.logger.Warn("no images detected")

	return false, nil
}

func convertContainerPorts(ports []testenvironmentv1alpha1.PortSpec) (result []corev1.ContainerPort) {
	for _, port := range ports {
		result = append(result, corev1.ContainerPort{
			Name:          port.Name,
			ContainerPort: int32(port.Port),
		})
	}

	return
}

func getLabels(b *testenvironmentv1alpha1.Build, name string, version bool) map[string]string {
	labels := map[string]string{}
	labels["app"] = b.Name
	labels["component"] = name

	if version {
		labels["version"] = b.Spec.Git.Ref[:6]
	}

	return labels
}
