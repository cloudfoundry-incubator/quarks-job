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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	outputPersistDirName      = "output-persist-dir"
	outputPersistDirMountPath = "/mnt/output-persist/"
	serviceAccountName        = "persist-output-service-account"
	mountPath                 = "/mnt/quarks/"
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
func (j jobCreatorImpl) Create(ctx context.Context, eJob ejv1.ExtendedJob, namespace string) (retry bool, err error) {
	template := eJob.Spec.Template.DeepCopy()

	// Create a service account for the pod
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	// Bind read only role to the service account
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

	err = j.client.Create(ctx, serviceAccount)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return false, errors.Wrapf(err, "could not create service account for pod in ejob %s.", eJob.Name)
		}
	}

	err = j.client.Create(ctx, roleBinding)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return false, errors.Wrapf(err, "could not create role binding for pod in ejob '%s'", eJob.Name)
		}
	}

	// Fetch all secrets
	tokenSecret := corev1.Secret{}
	secretList := &corev1.SecretList{}
	err = j.client.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return false, errors.Wrapf(err, "could not get secret list related to ejob %s.", eJob.Name)
	}
	for _, secret := range secretList.Items {
		annotations := secret.GetAnnotations()
		annotation, ok := annotations["kubernetes.io/service-account.name"]
		if ok {
			if annotation == serviceAccountName {
				tokenSecret = secret
				break
			}
		}
	}

	// Mount service account token on container
	serviceAccountVolumeName := names.Sanitize(fmt.Sprintf("%s-%s", serviceAccount.Name, tokenSecret.Name))
	serviceAccountVolume := corev1.Volume{
		Name: serviceAccountVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tokenSecret.Name,
				DefaultMode: pointers.Int32(0644),
			},
		},
	}
	serviceAccountVolumeMount := corev1.VolumeMount{
		Name:      serviceAccountVolumeName,
		ReadOnly:  true,
		MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
	}

	// Set serviceaccount to the container
	template.Spec.Volumes = append(template.Spec.Volumes, serviceAccountVolume)

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
	for containerIndex, container := range template.Spec.Containers {

		// Add pod volume specs to the pod
		podVolumeSpec := corev1.Volume{
			Name:         names.Sanitize(fmt.Sprintf("%s%s", "output-", container.Name)),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}
		template.Spec.Volumes = append(template.Spec.Volumes, podVolumeSpec)

		// Add container volume specs to continer
		containerVolumeMountSpec := corev1.VolumeMount{
			Name:      names.Sanitize(fmt.Sprintf("%s%s", "output-", container.Name)),
			MountPath: mountPath,
		}
		template.Spec.Containers[containerIndex].VolumeMounts = append(template.Spec.Containers[containerIndex].VolumeMounts, containerVolumeMountSpec)

		// Add container volume spec to output persist container
		containerVolumeMountSpec.MountPath = filepath.Join(mountPath, container.Name)
		outputPersistContainer.VolumeMounts = append(outputPersistContainer.VolumeMounts, containerVolumeMountSpec)
	}

	// Add output persist container to the pod template
	template.Spec.Containers = append(template.Spec.Containers, outputPersistContainer)

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels[ejv1.LabelEJobName] = eJob.Name

	err = j.store.SetSecretReferences(ctx, eJob.Namespace, &template.Spec)
	if err != nil {
		return
	}

	configMaps, err := reference.GetConfigMapsReferencedBy(eJob)
	if err != nil {
		return
	}

	configMap := &corev1.ConfigMap{}
	for configMapName := range configMaps {
		err = j.client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: eJob.Namespace}, configMap)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to configMap '%s' not found", eJob.Name, configMapName)
				// we want to requeue the job without error
				retry = true
				err = nil
			}
			return
		}
	}

	secrets, err := reference.GetSecretsReferencedBy(ctx, j.client, eJob)
	if err != nil {
		return
	}

	secret := &corev1.Secret{}
	for secretName := range secrets {
		err = j.client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: eJob.Namespace}, secret)
		if err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to secret '%s' not found", eJob.Name, secretName)
				// we want to requeue the job without error
				retry = true
				err = nil
			}
			return
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
		Spec: batchv1.JobSpec{
			Template:     *template,
			BackoffLimit: pointers.Int32(2),
		},
	}

	err = j.setOwnerReference(&eJob, job, j.scheme)
	if err != nil {
		return false, ctxlog.WithEvent(&eJob, "SetOwnerReferenceError").Errorf(ctx, "failed to set owner reference on job for '%s': %s", eJob.Name, err)
	}

	err = j.client.Create(ctx, job)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctxlog.WithEvent(&eJob, "AlreadyRunning").Infof(ctx, "Skip '%s': already running", eJob.Name)
			// we don't want to requeue the job
			return retry, nil
		}
		retry = true
	}

	return
}
