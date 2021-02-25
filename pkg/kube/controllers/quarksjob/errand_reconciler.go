package quarksjob

import (
	"context"
	"time"

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
	// ReconcileSkipDuration is the duration of merging consecutive triggers.
	ReconcileSkipDuration = 10 * time.Second
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
		ctx:        ctx,
		client:     mgr.GetClient(),
		config:     config,
		scheme:     mgr.GetScheme(),
		jobCreator: jc,
	}
}

// ErrandReconciler implements the Reconciler interface.
type ErrandReconciler struct {
	ctx        context.Context
	client     client.Client
	config     *config.Config
	scheme     *runtime.Scheme
	jobCreator JobCreator
}

// Reconcile starts jobs for quarks jobs of the type errand with Run being set to 'now' manually.
func (r *ErrandReconciler) Reconcile(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
	qJob := &qjv1a1.QuarksJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling errand job '%s'", request.NamespacedName)
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

	if qJob.Status.LastReconcile == nil {
		now := metav1.Now()
		qJob.Status.LastReconcile = &now

		err := r.client.Status().Update(ctx, qJob)
		if err != nil {
			return reconcile.Result{}, ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on job '%s' (%v): %s", qJob.GetNamespacedName(), qJob.ResourceVersion, err)
		}
		ctxlog.Infof(ctx, "Meltdown started for '%s'", request.NamespacedName)

		return reconcile.Result{RequeueAfter: ReconcileSkipDuration}, nil
	}

	if meltdown.NewWindow(ReconcileSkipDuration, qJob.Status.LastReconcile).Contains(time.Now()) {
		ctxlog.Infof(ctx, "Meltdown in progress for '%s'", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	ctxlog.Infof(ctx, "Meltdown ended for '%s'", request.NamespacedName)
	qJob.Status.LastReconcile = nil
	err := r.client.Status().Update(ctx, qJob)
	if err != nil {
		return reconcile.Result{}, ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on job '%s' (%v): %s", qJob.GetNamespacedName(), qJob.ResourceVersion, err)
	}

	if qJob.Spec.Trigger.Strategy == qjv1a1.TriggerNow {
		// Set Strategy back to manual for errand jobs.
		qJob.Spec.Trigger.Strategy = qjv1a1.TriggerManual
		if err := r.client.Update(ctx, qJob); err != nil {
			return reconcile.Result{}, ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to revert to 'trigger.strategy=manual' on job '%s': %s", qJob.GetNamespacedName(), err)
		}
	}

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
			_ = ctxlog.WithEvent(qJob, "UpdateError").Errorf(ctx, "Failed to traverse to 'trigger.strategy=done' on job '%s': %s", qJob.GetNamespacedName(), err)
			return reconcile.Result{Requeue: false}, nil
		}
	}

	return reconcile.Result{}, nil
}
