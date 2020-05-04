package quarksjob

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

const (
	outputPersistDirName      = "output-persist-dir"
	outputPersistDirMountPath = "/mnt/output-persist/"
	mountPath                 = "/mnt/quarks/"
	// EnvNamespace is the namespace in which the jobs run, used by
	// persist-output to create the secrets
	EnvNamespace = "NAMESPACE"
)

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewJobCreator returns a new job creator
func NewJobCreator(client crc.Client, scheme *runtime.Scheme, f setOwnerReferenceFunc, config *config.Config, store vss.VersionedSecretStore) JobCreator {
	return jobCreatorImpl{
		client:            client,
		scheme:            scheme,
		setOwnerReference: f,
		config:            config,
		store:             store,
	}
}

// JobCreator is the interface that wraps the basic Create method.
type JobCreator interface {
	Create(ctx context.Context, qJob qjv1a1.QuarksJob) (retry bool, err error)
}

type jobCreatorImpl struct {
	client            crc.Client
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
	config            *config.Config
	store             vss.VersionedSecretStore
}

// Create satisfies the JobCreator interface. It creates a Job to complete ExJob. It returns the
// retry if one of the references are not present.
func (j jobCreatorImpl) Create(ctx context.Context, qJob qjv1a1.QuarksJob) (bool, error) {
	namespace := qJob.Namespace
	template := qJob.Spec.Template.DeepCopy()

	serviceAccount, err := j.getServiceAccountName(ctx, namespace)
	if err != nil {
		return false, err
	}

	serviceAccountVolume, serviceAccountVolumeMount, err := j.serviceAccountMount(ctx, namespace, serviceAccount)
	if err != nil {
		return false, err
	}

	// Set serviceaccount to the container
	template.Spec.Template.Spec.Volumes = append(template.Spec.Template.Spec.Volumes, *serviceAccountVolume)

	ctxlog.Debugf(ctx, "Add persist output container, using DOCKER_IMAGE_TAG=%s", config.GetOperatorDockerImage())
	// Create a container for persisting output
	outputPersistContainer := corev1.Container{
		Name:            "output-persist",
		Image:           config.GetOperatorDockerImage(),
		ImagePullPolicy: config.GetOperatorImagePullPolicy(),
		Args:            []string{"persist-output"},
		Env: []corev1.EnvVar{
			{
				Name:  EnvNamespace,
				Value: namespace,
			},
		},
		VolumeMounts: []corev1.VolumeMount{*serviceAccountVolumeMount},
	}

	// Loop through containers and add quarks logging volume specs.
	for containerIndex, container := range template.Spec.Template.Spec.Containers {

		// Add pod volume specs to the pod
		podVolumeSpec := corev1.Volume{
			Name:         names.Sanitize(fmt.Sprintf("%s%s", "output-", container.Name)),
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}
		template.Spec.Template.Spec.Volumes = append(template.Spec.Template.Spec.Volumes, podVolumeSpec)

		// Add container volume specs to container
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
	template.Spec.Template.Labels[qjv1a1.LabelQJobName] = qJob.Name

	if err := j.store.SetSecretReferences(ctx, qJob.Namespace, &template.Spec.Template.Spec); err != nil {
		return false, err
	}

	// Validate quarks job configmap and secrets references
	err = j.validateReferences(ctx, qJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Requeue the job without error.
			return true, nil
		}
		return false, err
	}

	// Create k8s job
	name, err := names.JobName(qJob.Name)
	if err != nil {
		return false, errors.Wrapf(err, "could not generate job name for qJob '%s'", qJob.GetNamespacedName())
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: qJob.Namespace,
			Labels:    map[string]string{qjv1a1.LabelQJobName: qJob.Name},
		},
		Spec: template.Spec,
	}

	if err := j.setOwnerReference(&qJob, job, j.scheme); err != nil {
		return false, ctxlog.WithEvent(&qJob, "SetOwnerReferenceError").Errorf(ctx, "failed to set owner reference on job for '%s': %s", qJob.GetNamespacedName(), err)
	}

	if err := j.client.Create(ctx, job); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ctxlog.WithEvent(&qJob, "AlreadyRunning").Infof(ctx, "Skip '%s': already running", qJob.GetNamespacedName())
			// Don't requeue the job.
			return false, nil
		}
		return true, err
	}

	return false, nil
}

func (j jobCreatorImpl) validateReferences(ctx context.Context, qJob qjv1a1.QuarksJob) error {
	configMaps := reference.ReferencedConfigMaps(qJob)
	configMap := &corev1.ConfigMap{}
	for configMapName := range configMaps {
		if err := j.client.Get(ctx, crc.ObjectKey{Name: configMapName, Namespace: qJob.Namespace}, configMap); err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to configMap '%s' not found", qJob.GetNamespacedName(), configMapName)
			}
			return err
		}
	}

	secrets := reference.ReferencedSecrets(qJob)
	secret := &corev1.Secret{}
	for secretName := range secrets {
		if err := j.client.Get(ctx, crc.ObjectKey{Name: secretName, Namespace: qJob.Namespace}, secret); err != nil {
			if apierrors.IsNotFound(err) {
				ctxlog.Debugf(ctx, "Skip create job '%s' due to secret '%s' not found", qJob.GetNamespacedName(), secretName)
			}
			return err
		}
	}
	return nil
}
