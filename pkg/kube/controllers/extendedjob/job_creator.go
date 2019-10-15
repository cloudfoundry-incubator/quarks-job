package extendedjob

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

const (
	outputPersistDirName          = "output-persist-dir"
	outputPersistDirMountPath     = "/mnt/output-persist/"
	serviceAccountName            = "persist-output-service-account"
	serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
	mountPath                     = "/mnt/quarks/"
)

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewJobCreator returns a new job creator
func NewJobCreator(client crc.Client, scheme *runtime.Scheme, f setOwnerReferenceFunc, store vss.VersionedSecretStore) JobCreator {
	return jobCreatorImpl{
		client:            client,
		scheme:            scheme,
		setOwnerReference: f,
		store:             store,
	}
}

// JobCreator is the interface that wraps the basic Create method.
type JobCreator interface {
	Create(ctx context.Context, eJob ejv1.ExtendedJob, namespace string) (retry bool, err error)
}

type jobCreatorImpl struct {
	client            crc.Client
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
	store             vss.VersionedSecretStore
}

// Create satisfies the JobCreator interface. It creates a Job to complete ExJob. It returns the
// retry if one of the references are not present.
func (j jobCreatorImpl) Create(ctx context.Context, eJob ejv1.ExtendedJob, namespace string) (bool, error) {
	template := eJob.Spec.Template.DeepCopy()

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
			return false, errors.Wrapf(err, "could not create service account for pod in ejob %s.", eJob.Name)
		}
	}

	if err := j.client.Create(ctx, roleBinding); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return false, errors.Wrapf(err, "could not create role binding for pod in ejob '%s'", eJob.Name)
		}
	}

	var createdServiceAccount corev1.ServiceAccount
	if err := j.client.Get(ctx, crc.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &createdServiceAccount); err != nil {
		return false, errors.Wrapf(err, "could not get %s", eJob.Name)
	}

	if len(createdServiceAccount.Secrets) == 0 {
		return false, fmt.Errorf("missing service account secret for '%s'", serviceAccountName)
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

	// Set serviceaccount to the container
	template.Spec.Template.Spec.Volumes = append(template.Spec.Template.Spec.Volumes, serviceAccountVolume)

	image := config.GetOperatorDockerImage()
	image = strings.Replace(image, "quarks-job", "cf-operator", 1)
	// Create a container for persisting output
	outputPersistContainer := corev1.Container{
		Name:            "output-persist",
		Image:           image,
		ImagePullPolicy: config.GetOperatorImagePullPolicy(),
		Command:         []string{"/usr/bin/dumb-init", "--"},
		Args: []string{
			"/bin/sh",
			"-xc",
			"cf-operator util persist-output",
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvNamespace,
				Value: namespace,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			serviceAccountVolumeMount,
		},
	}

	// Loop through containers and add quarks logging volume specs.
	for containerIndex, container := range template.Spec.Template.Spec.Containers {

		// Add pod volume specs to the pod
		podVolumeSpec := corev1.Volume{
			Name:         names.Sanitize(fmt.Sprintf("%s%s", "output-", container.Name)),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}
		template.Spec.Template.Spec.Volumes = append(template.Spec.Template.Spec.Volumes, podVolumeSpec)

		// Add container volume specs to continer
		containerVolumeMountSpec := corev1.VolumeMount{
			Name:      names.Sanitize(fmt.Sprintf("%s%s", "output-", container.Name)),
			MountPath: mountPath,
		}
		template.Spec.Template.Spec.Containers[containerIndex].VolumeMounts = append(template.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, containerVolumeMountSpec)

		// Add container volume spec to output persist container
		containerVolumeMountSpec.MountPath = filepath.Join(mountPath, container.Name)
		outputPersistContainer.VolumeMounts = append(outputPersistContainer.VolumeMounts, containerVolumeMountSpec)
	}

	// Add output persist container to the pod template
	template.Spec.Template.Spec.Containers = append(template.Spec.Template.Spec.Containers, outputPersistContainer)

	if template.Spec.Template.Labels == nil {
		template.Spec.Template.Labels = map[string]string{}
	}
	template.Spec.Template.Labels[ejv1.LabelEJobName] = eJob.Name

	if err := j.store.SetSecretReferences(ctx, eJob.Namespace, &template.Spec.Template.Spec); err != nil {
		return false, err
	}

	configMaps, err := reference.GetConfigMapsReferencedBy(eJob)
	if err != nil {
		return false, err
	}

	configMap := &corev1.ConfigMap{}
	for configMapName := range configMaps {
		if err := j.client.Get(ctx, crc.ObjectKey{Name: configMapName, Namespace: eJob.Namespace}, configMap); err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to configMap '%s' not found", eJob.Name, configMapName)
				// Requeue the job without error.
				return true, nil
			}
			return false, err
		}
	}

	secrets, err := reference.GetSecretsReferencedBy(ctx, j.client, eJob)
	if err != nil {
		return false, err
	}

	secret := &corev1.Secret{}
	for secretName := range secrets {
		if err := j.client.Get(ctx, crc.ObjectKey{Name: secretName, Namespace: eJob.Namespace}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to secret '%s' not found", eJob.Name, secretName)
				// Requeue the job without error.
				return true, nil
			}
			return false, err
		}
	}

	name, err := names.JobName(eJob.Name)
	if err != nil {
		return false, errors.Wrapf(err, "could not generate job name for eJob '%s'", eJob.Name)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: eJob.Namespace,
			Labels:    map[string]string{ejv1.LabelExtendedJob: "true"},
		},
		Spec: template.Spec,
	}

	if err := j.setOwnerReference(&eJob, job, j.scheme); err != nil {
		return false, ctxlog.WithEvent(&eJob, "SetOwnerReferenceError").Errorf(ctx, "failed to set owner reference on job for '%s': %s", eJob.Name, err)
	}

	if err := j.client.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctxlog.WithEvent(&eJob, "AlreadyRunning").Infof(ctx, "Skip '%s': already running", eJob.Name)
			// Don't requeue the job.
			return false, nil
		}
		return true, err
	}

	return false, nil
}
