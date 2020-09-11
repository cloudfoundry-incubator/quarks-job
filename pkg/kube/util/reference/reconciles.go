package reference

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-job/pkg/kube/apis"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/podref"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// GetReconciles returns reconciliation requests for the QuarksJobs
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconciles(ctx context.Context, client crc.Client, object apis.Object) ([]reconcile.Request, error) {
	objReferencedBy := func(parent qjv1a1.QuarksJob) (bool, error) {
		var (
			objectReferences map[string]bool
			err              error
			name             string
			versionedSecret  bool
		)

		switch object := object.(type) {
		case *corev1.ConfigMap:
			objectReferences = podref.GetConfMapRefFromPod(parent.Spec.Template.Spec.Template.Spec)
			name = object.Name
		case *corev1.Secret:
			objectReferences = podref.GetSecretRefFromPodSpec(parent.Spec.Template.Spec.Template.Spec)
			name = object.Name
			versionedSecret = vss.IsVersionedSecret(*object)
		default:
			return false, errors.New("can't get reconciles for unknown object type; supported types are ConfigMap and Secret")
		}

		if err != nil {
			return false, errors.Wrap(err, "error listing references")
		}

		if versionedSecret {
			keys := make([]string, len(objectReferences))
			i := 0
			for k := range objectReferences {
				keys[i] = k
				i++
			}
			ok := vss.ContainsSecretName(keys, name)
			return ok, nil
		}

		_, ok := objectReferences[name]
		return ok, nil
	}

	namespace := object.GetNamespace()
	result := []reconcile.Request{}

	log.Debugf(ctx, "Searching QuarksJobs for references to '%s/%s' ", namespace, object.GetName())
	quarksJobs := &qjv1a1.QuarksJobList{}
	err := client.List(ctx, quarksJobs, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list QuarksJobs for reconciles")
	}

	for _, qJob := range quarksJobs.Items {
		if !(qJob.Spec.UpdateOnConfigChange && qJob.IsAutoErrand()) {
			continue
		}
		isRef, err := objReferencedBy(qJob)
		if err != nil {
			return nil, err
		}

		if isRef {
			result = append(result, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      qJob.Name,
					Namespace: qJob.Namespace,
				}})
		}
	}

	return result, nil
}

// SkipReconciles returns true if the object is stale, and shouldn't be enqueued for reconciliation
// The object can be a ConfigMap or a Secret
func SkipReconciles(ctx context.Context, client crc.Client, object apis.Object) bool {
	var newResourceVersion string

	switch object := object.(type) {
	case *corev1.ConfigMap:
		cm := &corev1.ConfigMap{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, cm)
		if err != nil {
			log.Errorf(ctx, "Failed to get ConfigMap '%s/%s': %s", object.Namespace, object.Name, err)
			return true
		}

		newResourceVersion = cm.ResourceVersion
	case *corev1.Secret:
		s := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, s)
		if err != nil {
			log.Errorf(ctx, "Failed to get Secret '%s/%s': %s", object.Namespace, object.Name, err)
			return true
		}

		newResourceVersion = s.ResourceVersion
	default:
		return false
	}

	if object.GetResourceVersion() != newResourceVersion {
		log.Debugf(ctx, "skip reconcile request for old resource version of '%s'", object.GetName())
		return true
	}
	return false
}
