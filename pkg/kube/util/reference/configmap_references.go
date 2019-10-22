package reference

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
)

// GetConfigMapsReferencedBy returns a list of all names for ConfigMaps referenced by the object
// The object can be an ExtendedStatefulSet, an ExtendedeJob or a BOSHDeployment
func GetConfigMapsReferencedBy(object interface{}) (map[string]bool, error) {
	// Figure out the type of object
	switch object := object.(type) {
	case ejv1.ExtendedJob:
		return getConfMapRefFromEJob(object), nil
	default:
		return nil, errors.New("can't get config map references for unkown type; supported types are BOSHDeployment, ExtendedJob and ExtendedStatefulSet")
	}
}

func getConfMapRefFromEJob(object ejv1.ExtendedJob) map[string]bool {
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
