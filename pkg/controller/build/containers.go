package build

import (
	"fmt"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (br *buildReconciler) reconcileContainers(force bool) error {
	// Loop over services and create deployments and services
	for _, service := range br.environment.Spec.Containers {
		if err := br.reconcileContainerDeployment(service, force); err != nil {
			return err
		}
		if err := br.reconcileContainerService(service); err != nil {
			return err
		}
	}

	return nil
}

func (br *buildReconciler) reconcileContainerDeployment(
	service testenvironmentv1alpha1.ContainerSpec,
	force bool,
) error {
	name := fmt.Sprintf("%s-container", service.Name)
	logger := br.logger.WithField("container", name)

	var terminationGracePeriodSeconds int64
	var configMapRefOptional = false

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: br.namespace,
			Labels:    getLabels(br.build, name, true),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(br.build, name, false),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: getLabels(br.build, name, true)},
				Spec: corev1.PodSpec{
					ServiceAccountName:            br.serviceAccountName,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					NodeSelector:                  br.environment.Spec.NodeSelector,
					Containers: []v1.Container{
						{
							Name:            service.Name,
							Image:           br.build.Spec.Image,
							ImagePullPolicy: v1.PullIfNotPresent,
							Args:            service.Args,
							EnvFrom: []v1.EnvFromSource{
								{
									ConfigMapRef: &v1.ConfigMapEnvSource{
										LocalObjectReference: v1.LocalObjectReference{Name: fmt.Sprintf("%ssharedenv", options.BuildPrefix)},
										Optional:             &configMapRefOptional,
									},
								},
							},
							Env:            service.Env,
							Ports:          convertContainerPorts(service.Ports),
							ReadinessProbe: service.ReadinessProbe,
							LivenessProbe:  service.LivenessProbe,
							Resources:      service.Resources,
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &appsv1.Deployment{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating container")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	// Only update deployment if the current running image is wrong
	if len(found.Spec.Template.Spec.Containers) != 1 ||
		found.Spec.Template.Spec.Containers[0].Image != br.build.Spec.Image {
		logger.Info("updating container")
		found.Spec.Template.Spec = deploy.Spec.Template.Spec
		return br.r.Update(br.ctx, found)
	}

	return nil
}

// nolint: dupl
func (br *buildReconciler) reconcileContainerService(service testenvironmentv1alpha1.ContainerSpec) error {
	ports := []corev1.ServicePort{}
	for _, port := range service.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:     port.Name,
			Protocol: corev1.ProtocolTCP,
			Port:     int32(port.Port),
		})
	}

	if len(ports) == 0 {
		return nil
	}

	name := fmt.Sprintf("%s-container", service.Name)
	logger := br.logger.WithField("container-service", name)

	deploy := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: br.namespace,
			Labels:    getLabels(br.build, name, true),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: getLabels(br.build, name, false),
			Ports:    ports,
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating container service")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
