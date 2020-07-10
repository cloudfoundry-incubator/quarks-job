package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-job/pkg/kube/apis"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// ReconcileType lists all the types of reconciliations we can return,
// for controllers that have types that can reference ConfigMaps or Secrets
type ReconcileType int

const (
	// ReconcileForQuarksJob represents the QuarksJob CRD
	ReconcileForQuarksJob = iota
)

func (r ReconcileType) String() string {
	return [...]string{
		"QuarksJob",
	}[r]
}

// GetReconciles returns reconciliation requests for the QuarksJobs
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconciles(ctx context.Context, client crc.Client, reconcileType ReconcileType, object apis.Object) ([]reconcile.Request, error) {
	objReferencedBy := func(parent qjv1a1.QuarksJob) (bool, error) {
		var (
			objectReferences map[string]bool
			err              error
			name             string
			versionedSecret  bool
		)

		switch object := object.(type) {
		case *corev1.ConfigMap:
			objectReferences = ReferencedConfigMaps(parent)
			name = object.Name
		case *corev1.Secret:
			objectReferences = ReferencedSecrets(parent)
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

	switch reconcileType {
	case ReconcileForQuarksJob:
		quarksJobs, err := listQuarksJobs(ctx, client, namespace)
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
	default:
		return nil, fmt.Errorf("unkown reconcile type %s", reconcileType.String())
	}

	return result, nil
}

func listQuarksJobs(ctx context.Context, client crc.Client, namespace string) (*qjv1a1.QuarksJobList, error) {
	log.Debugf(ctx, "Listing QuarksJobs in namespace '%s'", namespace)
	result := &qjv1a1.QuarksJobList{}
	err := client.List(ctx, result, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list QuarksJobs")
	}

	return result, nil
}
