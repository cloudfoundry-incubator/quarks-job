package quarksjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

const (
	serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
)

func (j jobCreatorImpl) serviceAccountMount(ctx context.Context, namespace string, serviceAccountName string) (*corev1.Volume, *corev1.VolumeMount, error) {
	var createdServiceAccount corev1.ServiceAccount
	if err := j.client.Get(ctx, crc.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &createdServiceAccount); err != nil {
		return nil, nil, errors.Wrapf(err, "could not get service account '%s'", serviceAccountName)
	}

	if len(createdServiceAccount.Secrets) == 0 {
		return nil, nil, fmt.Errorf("missing service account secret for '%s'", serviceAccountName)
	}
	tokenSecretName := createdServiceAccount.Secrets[0].Name

	// Mount service account token on container
	serviceAccountVolumeName := names.Sanitize(fmt.Sprintf("%s-%s", serviceAccountName, tokenSecretName))
	serviceAccountVolume := corev1.Volume{
		Name: serviceAccountVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tokenSecretName,
				DefaultMode: pointers.Int32(0644),
			},
		},
	}
	serviceAccountVolumeMount := corev1.VolumeMount{
		Name:      serviceAccountVolumeName,
		ReadOnly:  true,
		MountPath: serviceAccountSecretMountPath,
	}

	return &serviceAccountVolume, &serviceAccountVolumeMount, nil
}
