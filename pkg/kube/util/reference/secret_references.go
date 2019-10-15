package reference

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
)

// GetSecretsReferencedBy returns a list of all names for Secrets referenced by the object
// The object can be an ExtendedStatefulSet, an ExtendedJob or a BOSHDeployment
func GetSecretsReferencedBy(ctx context.Context, client crc.Client, object interface{}) (map[string]bool, error) {
	switch object := object.(type) {
	case ejv1.ExtendedJob:
		return getSecretRefFromEJob(object), nil
	default:
		return nil, errors.New("can't get secret references for unknown type; supported types are BOSHDeployment, ExtendedJob and ExtendedStatefulSet")
	}
}

func getSecretRefFromEJob(object ejv1.ExtendedJob) map[string]bool {
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
