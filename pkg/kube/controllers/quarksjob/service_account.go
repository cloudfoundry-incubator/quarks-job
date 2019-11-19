package quarksjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

const (
	serviceAccountName            = "persist-output-service-account"
	serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
)

func (j jobCreatorImpl) createdServiceAccount(ctx context.Context, namespace string) (*corev1.Volume, *corev1.VolumeMount, error) {
	// Create a service account for the pod
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	// Bind the persist-output service account to the cluster-admin ClusterRole. Notice that the
	// RoleBinding is namespaced as opposed to ClusterRoleBinding which would give the service account
	// unrestricted permissions to any namespace.
	roleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-admin-role",
			Namespace: namespace,
		},
		Subjects: []v1.Subject{
			{
				Kind:      v1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: v1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if err := j.client.Create(ctx, serviceAccount); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, nil, errors.Wrapf(err, "could not create service account")
		}
	}

	if err := j.client.Create(ctx, roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, nil, errors.Wrapf(err, "could not create role binding")
		}
	}

	var createdServiceAccount corev1.ServiceAccount
	if err := j.client.Get(ctx, crc.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &createdServiceAccount); err != nil {
		return nil, nil, errors.Wrapf(err, "could not get service account '%s'", serviceAccountName)
	}

	if len(createdServiceAccount.Secrets) == 0 {
		return nil, nil, fmt.Errorf("missing service account secret for '%s'", serviceAccountName)
	}
	tokenSecretName := createdServiceAccount.Secrets[0].Name

	// Mount service account token on container
	serviceAccountVolumeName := names.Sanitize(fmt.Sprintf("%s-%s", serviceAccount.Name, tokenSecretName))
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
