package build

import (
	"fmt"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (br *buildReconciler) reconcileServices(force bool) error {
	// Loop over services and create deployments and services
	for _, service := range br.environment.Spec.Services {
		if err := br.reconcileServiceDeployment(service, force); err != nil {
			return err
		}
		if err := br.reconcileServiceService(service); err != nil {
			return err
		}
	}

	return nil
}

// nolint: gocyclo
func (br *buildReconciler) reconcileServiceDeployment(service testenvironmentv1alpha1.ServiceSpec, force bool) error {
	name := fmt.Sprintf("%s-service", service.Name)
	logger := br.logger.WithField("service", name)

	var terminationGracePeriodSeconds int64

	var volumes []v1.Volume
	for id := range service.SharedDirs {
		volumes = append(volumes, v1.Volume{
			Name: fmt.Sprintf("shareddir-%d", id),
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		})
	}

	var volumeMounts []v1.VolumeMount
	for id, sharedDir := range service.SharedDirs {
		volumeMounts = append(volumeMounts, v1.VolumeMount{
			Name:      fmt.Sprintf("shareddir-%d", id),
			MountPath: sharedDir,
		})
	}

	var initContainers []v1.Container
	for _, initContainer := range service.InitContainers {
		initContainers = append(initContainers, v1.Container{
			Name:            initContainer.Name,
			Image:           initContainer.Image,
			ImagePullPolicy: v1.PullIfNotPresent,
			Env:             initContainer.Env,
			Command:         initContainer.Command,
			Args:            initContainer.Args,
			VolumeMounts:    volumeMounts,
		})
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", service.Name),
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
					InitContainers:                initContainers,
					Containers: []v1.Container{
						{
							Name:            service.Name,
							Image:           service.Image,
							ImagePullPolicy: v1.PullIfNotPresent,
							Args:            service.Args,
							Env:             service.Env,
							Ports:           convertContainerPorts(service.Ports),
							ReadinessProbe:  service.ReadinessProbe,
							LivenessProbe:   service.LivenessProbe,
							Resources:       service.Resources,
							VolumeMounts:    volumeMounts,
						},
					},
					Volumes: volumes,
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
		logger.Info("creating service")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	if force && !service.Protected {
		logger.Info("recreating service")
		if err := br.r.Delete(br.ctx, found); err != nil && !errors.IsNotFound(err) {
			return err
		}
		time.Sleep(1 * time.Second)
		return br.r.Create(br.ctx, deploy)
	}

	return nil
}

// nolint: dupl
func (br *buildReconciler) reconcileServiceService(service testenvironmentv1alpha1.ServiceSpec) error {
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

	name := fmt.Sprintf("%s-service", service.Name)
	logger := br.logger.WithField("service-service", name)

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
		logger.Info("creating service service")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	return nil
}
