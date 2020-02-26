package database

import (
	"context"
	"fmt"
	"reflect"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileDatabase) reconcileDeployment(
	ctx context.Context,
	database *testenvironmentv1alpha1.Database,
	template *testenvironmentv1alpha1.DatabaseTemplate,
	options *Options,
) error {
	var terminationGracePeriodSeconds int64 = 60 * 3
	var replicas int32 = 1
	var replicationHistory int32
	labels := Labels(database)
	containers := make([]corev1.Container, 1)

	// Choose container configuration to use
	if database.Status.Phase == testenvironmentv1alpha1.DatabaseReady ||
		database.Status.Phase == testenvironmentv1alpha1.DatabaseClaimed {
		containers[0] = *serveContainer(database, template)
	} else {
		containers[0] = *restoreContainer(database, template)
	}

	// Choose replicas
	if database.Status.Phase == testenvironmentv1alpha1.DatabasePending ||
		database.Status.Phase == testenvironmentv1alpha1.DatabaseReady {
		// No replicas during pending and ready state
		replicas = 0
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      database.Name,
			Namespace: options.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			RevisionHistoryLimit: &replicationHistory,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName:            options.ServiceAccountName,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					NodeSelector:                  template.Spec.NodeSelector,
					Containers:                    containers,
					Volumes: []corev1.Volume{
						{
							Name: "postgres-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: database.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(database, deploy, r.scheme); err != nil {
		return err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, deploy)
	} else if err != nil {
		return err
	}

	// Use DeepEqual, we do a lot of container manipulation based on the database phase
	if !reflect.DeepEqual(deploy.Spec, found.Spec) {
		found.Spec = deploy.Spec
		return r.Update(ctx, found)
	}

	return nil
}

func baseContainer(
	database *testenvironmentv1alpha1.Database,
	template *testenvironmentv1alpha1.DatabaseTemplate,
) *corev1.Container {
	var image = fmt.Sprintf("postgres:%s", template.Spec.DatabaseVersion)

	return &corev1.Container{
		Image:           image,
		ImagePullPolicy: v1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{
			{
				Name:          "tcp-postgres",
				ContainerPort: int32(5432),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []v1.EnvVar{
			{Name: "POSTGRES_DB", Value: database.Status.DatabaseName},
			{Name: "POSTGRES_USER", Value: database.Status.Username},
			{Name: "POSTGRES_PASSWORD", Value: database.Status.Password},
			{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "postgres-data",
				MountPath: "/var/lib/postgresql/data",
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: int32(10),
			TimeoutSeconds:      int32(5),
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromString("tcp-postgres"),
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: int32(10),
			TimeoutSeconds:      int32(5),
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromString("tcp-postgres"),
				},
			},
		},
	}
}

func restoreContainer(
	database *testenvironmentv1alpha1.Database,
	template *testenvironmentv1alpha1.DatabaseTemplate,
) *corev1.Container {
	var container = *baseContainer(database, template)

	cpuQuantity, err := resource.ParseQuantity("2")
	if err != nil {
		panic(err)
	}
	memoryQuantity, err := resource.ParseQuantity("8Gi")
	if err != nil {
		panic(err)
	}
	cpuQuantityLimits, err := resource.ParseQuantity("3")
	if err != nil {
		panic(err)
	}
	memoryQuantityLimits, err := resource.ParseQuantity("10Gi")
	if err != nil {
		panic(err)
	}

	container.Name = "restore"
	container.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		},
		Limits: corev1.ResourceList{
			"cpu":    cpuQuantityLimits,
			"memory": memoryQuantityLimits,
		},
	}
	container.Command = []string{
		"docker-entrypoint.sh",
		"-c", "shared_buffers=1GB",
		"-c", "effective_cache_size=6GB",
		"-c", "work_mem=100MB",
		"-c", "maintenance_work_mem=1GB",
		"-c", "effective_io_concurrency=200",
		"-c", "random_page_cost=1",
		"-c", "fsync=off",
		"-c", "synchronous_commit=off",
		"-c", "wal_level=minimal",
		"-c", "full_page_writes=off",
		"-c", "wal_buffers=64MB",
		"-c", "max_wal_size=20GB",
		"-c", "max_wal_senders=0",
		"-c", "wal_keep_segments=0",
		"-c", "archive_mode=off",
		"-c", "autovacuum=off",
	}

	return &container
}

func serveContainer(
	database *testenvironmentv1alpha1.Database,
	template *testenvironmentv1alpha1.DatabaseTemplate,
) *corev1.Container {
	var container = *baseContainer(database, template)

	cpuQuantityRequests, err := resource.ParseQuantity("400m")
	if err != nil {
		panic(err)
	}

	cpuQuantity, err := resource.ParseQuantity("800m")
	if err != nil {
		panic(err)
	}
	memoryQuantity, err := resource.ParseQuantity("2Gi")
	if err != nil {
		panic(err)
	}

	container.Name = "serve"
	container.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    cpuQuantityRequests,
			"memory": memoryQuantity,
		},
		Limits: corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		},
	}
	container.Command = []string{
		"docker-entrypoint.sh",
		"-c", "shared_buffers=512MB",
		"-c", "effective_cache_size=1536MB",
		"-c", "work_mem=200MB",
		"-c", "effective_io_concurrency=200",
		"-c", "random_page_cost=1",
	}

	return &container
}
