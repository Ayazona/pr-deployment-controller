package build

import (
	"fmt"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/pr-deployment-controller/pkg/apis/testenvironment/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (br *buildReconciler) reconcileTasks(force bool) error {
	// Loop over tasks and create jobs
	for _, task := range br.environment.Spec.Tasks {
		if err := br.reconcileTask(task, force); err != nil {
			return err
		}
	}

	return nil
}

func (br *buildReconciler) reconcileTask(task testenvironmentv1alpha1.TaskSpec, force bool) error {
	name := fmt.Sprintf("%s-task", task.Name)
	logger := br.logger.WithField("job", task.Name)

	var terminationGracePeriodSeconds int64
	var configMapRefOptional = false

	deploy := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: br.namespace,
			Labels:    getLabels(br.build, name, true),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: getLabels(br.build, name, true)},
				Spec: corev1.PodSpec{
					ServiceAccountName:            br.serviceAccountName,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					RestartPolicy:                 corev1.RestartPolicyOnFailure,
					NodeSelector:                  br.environment.Spec.NodeSelector,
					Containers: []corev1.Container{
						{
							Name:  task.Name,
							Image: br.build.Spec.Image,
							Args:  task.Args,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%ssharedenv", br.options.BuildPrefix)},
										Optional:             &configMapRefOptional,
									},
								},
							},
							Env:       task.Env,
							Resources: task.Resources,
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(br.build, deploy, br.r.scheme); err != nil {
		return err
	}

	found := &batchv1.Job{}
	err := br.r.Get(br.ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating task")
		return br.r.Create(br.ctx, deploy)
	} else if err != nil {
		return err
	}

	if force {
		logger.Info("recreating task")
		if err := br.r.Delete(br.ctx, found); err != nil && !errors.IsNotFound(err) {
			return err
		}
		time.Sleep(1 * time.Second)
		return br.r.Create(br.ctx, deploy)
	}

	return nil
}
