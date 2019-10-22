// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultConfigMap for tests
func (c *Catalog) DefaultConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			name: "default-value",
		},
	}
}

// DefaultServiceAccount for tests
func (c *Catalog) DefaultServiceAccount(name string) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Secrets: []corev1.ObjectReference{
			{
				Name: name,
			},
		},
	}
}

// DefaultSecret for tests
func (c *Catalog) DefaultSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			name: "default-value",
		},
	}
}

// Sleep1hPodSpec defines a simple pod that sleeps 60*60s for testing
func (c *Catalog) Sleep1hPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: pointers.Int64(1),
		Containers: []corev1.Container{
			{
				Name:    "busybox",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
		},
	}
}

// DefaultPod defines a pod with a simple web server useful for testing
func (c *Catalog) DefaultPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// ConfigJobTemplate returns the spec with a given command for busybox
func (c *Catalog) ConfigJobTemplate() batchv1b1.JobTemplateSpec {
	one := int64(1)
	return batchv1b1.JobTemplateSpec{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"delete": "pod"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: &one,
					Volumes: []corev1.Volume{
						{
							Name: "secret1",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "secret1",
								},
							},
						},
						{
							Name: "configmap1",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "config1",
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: []string{"sleep", "1"},
							Env: []corev1.EnvVar{
								{Name: "REPLICAS", Value: "1"},
								{Name: "AZ_INDEX", Value: "1"},
								{Name: "POD_ORDINAL", Value: "0"},
							},
						},
					},
				},
			},
		},
	}
}

// ExJobPodTemplate returns the spec with a given output-persist container
func (c *Catalog) ExJobPodTemplate(cmd []string) batchv1b1.JobTemplateSpec {
	return batchv1b1.JobTemplateSpec{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: pointers.Int64(1),
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: cmd,
						},
						{
							Name:    "output-persist",
							Image:   "busybox",
							Command: cmd,
						},
					},
				},
			},
		},
	}
}

// FailingMultiContainerJobTemplate returns a spec with a given command for busybox and a second container which fails
func (c *Catalog) FailingMultiContainerJobTemplate(cmd []string) batchv1b1.JobTemplateSpec {
	return batchv1b1.JobTemplateSpec{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: pointers.Int64(1),
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: cmd,
						},
						{
							Name:    "failing",
							Image:   "busybox",
							Command: []string{"exit", "1"},
						},
					},
				},
			},
		},
	}
}

// CmdJobTemplate returns the spec with a given command for busybox
func (c *Catalog) CmdJobTemplate(cmd []string) batchv1b1.JobTemplateSpec {
	return batchv1b1.JobTemplateSpec{
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: pointers.Int64(1),
					Containers: []corev1.Container{
						{
							Name:    "busybox",
							Image:   "busybox",
							Command: cmd,
							Env: []corev1.EnvVar{
								{Name: "REPLICAS", Value: "1"},
								{Name: "AZ_INDEX", Value: "1"},
								{Name: "POD_ORDINAL", Value: "0"},
							},
						},
					},
				},
			},
		},
	}
}

// DefaultExtendedJob default values
func (c *Catalog) DefaultExtendedJob(name string) *ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.ExJobPodTemplate(cmd),
		},
	}
}

// DefaultExJobPod defines a pod with a simple web server and with a output-persist container
func (c *Catalog) DefaultExJobPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: c.Sleep1hExJobPodSpec(),
	}
}

// Sleep1hExJobPodSpec defines a simple pod that sleeps 60*60s for testing with a output-persist container
func (c *Catalog) Sleep1hExJobPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: pointers.Int64(1),
		Containers: []corev1.Container{
			{
				Name:    "busybox",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
			{
				Name:    "output-persist",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
		},
	}
}

// DefaultExtendedJobWithSucceededJob returns an ExtendedJob and a Job owned by it
func (c *Catalog) DefaultExtendedJobWithSucceededJob(name string) (*ejv1.ExtendedJob, *batchv1.Job, *corev1.Pod) {
	ejob := c.DefaultExtendedJob(name)
	backoffLimit := pointers.Int32(2)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-job",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       name,
					UID:        "",
					Controller: pointers.Bool(true),
				},
			},
		},
		Spec:   batchv1.JobSpec{BackoffLimit: backoffLimit},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	pod := c.DefaultExJobPod(name + "-pod")
	pod.Labels = map[string]string{
		"job-name": job.GetName(),
	}
	return ejob, job, &pod
}

// ErrandExtendedJob default values
func (c *Catalog) ErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.CmdJobTemplate(cmd),
		},
	}
}

// AutoErrandExtendedJob default values
func (c *Catalog) AutoErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			Template: c.CmdJobTemplate(cmd),
		},
	}
}
