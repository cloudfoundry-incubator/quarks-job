package quarksjob

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

var _ reconcile.Reconciler = &ErrandReconciler{}

const (
	// EnvKubeAz is set by available zone name
	EnvKubeAz = "KUBE_AZ"
	// EnvBoshAz is set by available zone name
	EnvBoshAz = "BOSH_AZ"
	// EnvReplicas describes the number of replicas in the quarks job
	EnvReplicas = "REPLICAS"
	// EnvCFOperatorAZIndex is set by available zone index
	EnvCFOperatorAZIndex = "AZ_INDEX"
	// EnvPodOrdinal is the pod's index
	EnvPodOrdinal = "POD_ORDINAL"
)

// NewErrandReconciler returns a new reconciler for errand jobs.
func NewErrandReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
	store vss.VersionedSecretStore,
) reconcile.Reconciler {
	jc := NewJobCreator(mgr.GetClient(), mgr.GetScheme(), f, config, store)

	return &ErrandReconciler{
		ctx:               ctx,
		client:            mgr.GetClient(),
		config:            config,
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
		jobCreator:        jc,
	}
}

// ErrandReconciler implements the Reconciler interface.
type ErrandReconciler struct {
	ctx               context.Context
	client            client.Client
	config            *config.Config
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
	jobCreator        JobCreator
}

// Reconcile starts jobs for quarks jobs of the type errand with Run being set to 'now' manually.
func (r *ErrandReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	qJob := &qjv1a1.QuarksJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling errand job ", request.NamespacedName)
	if err := r.client.Get(ctx, request.NamespacedName, qJob); err != nil {
		if apierrors.IsNotFound(err) {
			// Do not requeue, quarks job is probably deleted.
			ctxlog.Infof(ctx, "Failed to find quarks job '%s', not retrying: %s", request.NamespacedName, err)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get quarks job '%s': %s", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if meltdown.NewWindow(r.config.MeltdownDuration, qJob.Status.LastReconcile).Contains(time.Now()) {
		ctxlog.WithEvent(qJob, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", request.NamespacedName, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	if qJob.Spec.Trigger.Strategy == qjv1a1.TriggerNow {
		// Set Strategy back to manual for errand jobs.
		qJob.Spec.Trigger.Strategy = qjv1a1.TriggerManual
		if err := r.client.Update(ctx, qJob); err != nil {
			return reconcile.Result{}, ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to revert to 'trigger.strategy=manual' on job '%s': %s", qJob.GetNamespacedName(), err)
		}
	}

	r.injectContainerEnv(&qJob.Spec.Template.Spec.Template.Spec)
	if retry, err := r.jobCreator.Create(ctx, *qJob); err != nil {
		return reconcile.Result{}, ctxlog.WithEvent(qJob, "CreateJobError").Errorf(ctx, "Failed to create job '%s': %s", qJob.GetNamespacedName(), err)
	} else if retry {
		ctxlog.Infof(ctx, "Retrying to create job '%s'", qJob.GetNamespacedName())
		result := reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second * 5,
		}
		return result, nil
	}

	ctxlog.WithEvent(qJob, "CreateJob").Infof(ctx, "Created errand job for '%s'", qJob.GetNamespacedName())

	if qJob.Spec.Trigger.Strategy == qjv1a1.TriggerOnce {
		// Traverse Strategy into the final 'done' state.
		qJob.Spec.Trigger.Strategy = qjv1a1.TriggerDone
		if err := r.client.Update(ctx, qJob); err != nil {
			ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to traverse to 'trigger.strategy=done' on job '%s': %s", qJob.GetNamespacedName(), err)
			return reconcile.Result{Requeue: false}, nil
		}
	}

	// Reset status
	qJob.Status.Completed = false
	now := metav1.Now()
	qJob.Status.LastReconcile = &now
	err := r.client.Status().Update(ctx, qJob)
	if err != nil {
		ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on job '%s' (%v): %s", qJob.GetNamespacedName(), qJob.ResourceVersion, err)
		return reconcile.Result{Requeue: false}, nil
	}

	return reconcile.Result{}, nil
}

// injectContainerEnv injects AZ info to container envs
// errands always have an AZ_INDEX of 1
func (r *ErrandReconciler) injectContainerEnv(podSpec *corev1.PodSpec) {

	containers := []*corev1.Container{}
	for i := 0; i < len(podSpec.Containers); i++ {
		containers = append(containers, &podSpec.Containers[i])
	}
	for i := 0; i < len(podSpec.InitContainers); i++ {
		containers = append(containers, &podSpec.InitContainers[i])
	}
	for _, container := range containers {
		envs := container.Env

		// Default to zone 1, with 1 replica
		envs = upsertEnvs(envs, EnvCFOperatorAZIndex, "1")
		envs = upsertEnvs(envs, EnvReplicas, "1")
		envs = upsertEnvs(envs, EnvPodOrdinal, "0")

		container.Env = envs
	}
}

func upsertEnvs(envs []corev1.EnvVar, name string, value string) []corev1.EnvVar {
	for idx, env := range envs {
		if env.Name == name {
			envs[idx].Value = value
			return envs
		}
	}

	envs = append(envs, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return envs
}
