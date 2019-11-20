// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
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

// QuarksJobPodTemplate returns the spec with a given output-persist container
func (c *Catalog) QuarksJobPodTemplate(cmd []string) batchv1b1.JobTemplateSpec {
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

// CmdJobTemplate returns the job spec with a given command for busybox
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

// DefaultQuarksJob default values
func (c *Catalog) DefaultQuarksJob(name string) *qjv1a1.QuarksJob {
	cmd := []string{"sleep", "1"}
	return &qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerNow,
			},
			Template: c.QuarksJobPodTemplate(cmd),
		},
	}
}

// DefaultQuarksJobPod defines a pod with a simple web server and with a output-persist container
func (c *Catalog) DefaultQuarksJobPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: c.Sleep1hQuarksJobPodSpec(),
	}
}

// Sleep1hQuarksJobPodSpec defines a simple pod that sleeps 60*60s for testing with a output-persist container
func (c *Catalog) Sleep1hQuarksJobPodSpec() corev1.PodSpec {
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

// DefaultQuarksJobWithSucceededJob returns an QuarksJob and a Job owned by it
func (c *Catalog) DefaultQuarksJobWithSucceededJob(name string) (*qjv1a1.QuarksJob, *batchv1.Job, *corev1.Pod) {
	qJob := c.DefaultQuarksJob(name)
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
	pod := c.DefaultQuarksJobPod(name + "-pod")
	pod.Labels = map[string]string{
		"job-name": job.GetName(),
	}
	return qJob, job, &pod
}

// ErrandQuarksJob default values
func (c *Catalog) ErrandQuarksJob(name string) qjv1a1.QuarksJob {
	cmd := []string{"sleep", "1"}
	return qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerNow,
			},
			Template: c.CmdJobTemplate(cmd),
		},
	}
}

// AutoErrandQuarksJob default values
func (c *Catalog) AutoErrandQuarksJob(name string) qjv1a1.QuarksJob {
	cmd := []string{"sleep", "1"}
	return qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerOnce,
			},
			Template: c.CmdJobTemplate(cmd),
		},
	}
}

// DefaultOutputMap has default values to persist quarks job output to secrets
func (c *Catalog) DefaultOutputMap() qjv1a1.OutputMap {
	return qjv1a1.OutputMap{
		"busybox": qjv1a1.FilesToSecrets{
			"output.json": qjv1a1.SecretOptions{
				Name: "foo-busybox",
			},
			"output-nats.json": qjv1a1.SecretOptions{
				Name: "fake-nats",
			},
			"output-nuts.json": qjv1a1.SecretOptions{
				Name:      "bar-nuts",
				Versioned: true,
			},
		},
	}
}

// OutputQuarksJob default values
func (c *Catalog) OutputQuarksJob(name string) qjv1a1.QuarksJob {
	cmd := []string{"/bin/sh", "-c", "echo '{\"fake\": \"value\"}' | tee /mnt/quarks/output.json /mnt/quarks/output-nats.json /mnt/quarks/output-nuts.json"}
	return qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerNow,
			},
			Output: &qjv1a1.Output{
				WriteOnFailure: true,
				OutputMap:      c.DefaultOutputMap(),
			},
			Template: c.CmdJobTemplate(cmd),
		},
	}
}
