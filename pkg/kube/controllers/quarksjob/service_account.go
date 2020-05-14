package quarksjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	qjv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

const (
	serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
)

func (j jobCreatorImpl) getServiceAccountName(ctx context.Context, name string) (string, error) {
	var ns corev1.Namespace
	err := j.client.Get(ctx, crc.ObjectKey{Name: name}, &ns)
	if err != nil {
		return "", errors.Wrapf(err, "could not get namespace '%s'", name)
	}

	if acc, ok := ns.Labels[qjv1.LabelServiceAccount]; ok {
		return acc, nil
	}
	return "", fmt.Errorf("failed to retrieve persist output service account from namespace label '%s'", qjv1.LabelServiceAccount)
}

func (j jobCreatorImpl) serviceAccountMount(ctx context.Context, namespace string, serviceAccountName string) (*corev1.Volume, *corev1.VolumeMount, error) {
	var acct corev1.ServiceAccount
	if err := j.client.Get(ctx, crc.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &acct); err != nil {
		return nil, nil, errors.Wrapf(err, "could not get service account '%s'", serviceAccountName)
	}

	if len(acct.Secrets) == 0 {
		return nil, nil, fmt.Errorf("missing service account secret for '%s'", serviceAccountName)
	}
	tokenSecretName := acct.Secrets[0].Name

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
