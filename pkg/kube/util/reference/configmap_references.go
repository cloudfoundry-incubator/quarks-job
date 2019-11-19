package reference

import (
	corev1 "k8s.io/api/core/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
)

// ReferencedConfigMaps returns a list of all names for ConfigMaps referenced by the QuarksJob
func ReferencedConfigMaps(object qjv1a1.QuarksJob) map[string]bool {
	return getConfMapRefFromPod(object.Spec.Template.Spec.Template.Spec)
}

func getConfMapRefFromPod(object corev1.PodSpec) map[string]bool {
	result := map[string]bool{}

	// Look at all volumes
	for _, volume := range object.Volumes {
		if volume.VolumeSource.ConfigMap != nil {
			result[volume.VolumeSource.ConfigMap.Name] = true
		}
	}

	// Look at all init containers
	for _, container := range object.InitContainers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				result[envFrom.ConfigMapRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.ConfigMapKeyRef != nil {
				result[envVar.ValueFrom.ConfigMapKeyRef.Name] = true
			}
		}
	}

	// Look at all containers
	for _, container := range object.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				result[envFrom.ConfigMapRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.ConfigMapKeyRef != nil {
				result[envVar.ValueFrom.ConfigMapKeyRef.Name] = true
			}
		}
	}

	return result
}
