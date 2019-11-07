package reference

import (
	corev1 "k8s.io/api/core/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
)

// GetSecretsReferencesFromQuarksJob returns a list of all names for Secrets referenced by the QuarksJob
func GetSecretsReferencesFromQuarksJob(object qjv1a1.QuarksJob) map[string]bool {
	return getSecretRefFromPod(object.Spec.Template.Spec.Template.Spec)
}

func getSecretRefFromPod(object corev1.PodSpec) map[string]bool {
	result := map[string]bool{}

	// Look at all volumes
	for _, volume := range object.Volumes {
		if volume.VolumeSource.Secret != nil {
			result[volume.VolumeSource.Secret.SecretName] = true
		}
	}

	// Look at all init containers
	for _, container := range object.InitContainers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				result[envFrom.SecretRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				result[envVar.ValueFrom.SecretKeyRef.Name] = true
			}
		}
	}

	// Look at all containers
	for _, container := range object.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				result[envFrom.SecretRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				result[envVar.ValueFrom.SecretKeyRef.Name] = true
			}
		}
	}

	return result
}
