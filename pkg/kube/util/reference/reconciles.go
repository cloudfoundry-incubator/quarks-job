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
	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// ReconcileType lists all the types of reconciliations we can return,
// for controllers that have types that can reference ConfigMaps or Secrets
type ReconcileType int

const (
	// ReconcileForExtendedJob represents the ExtendedJob CRD
	ReconcileForExtendedJob = iota
)

func (r ReconcileType) String() string {
	return [...]string{
		"ExtendedJob",
	}[r]
}

// GetReconciles returns reconciliation requests for the BOSHDeployments, ExtendedJobs or ExtendedStatefulSets
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconciles(ctx context.Context, client crc.Client, reconcileType ReconcileType, object apis.Object) ([]reconcile.Request, error) {
	objReferencedBy := func(parent interface{}) (bool, error) {
		var (
			objectReferences map[string]bool
			err              error
			name             string
			versionedSecret  bool
		)

		switch object := object.(type) {
		case *corev1.ConfigMap:
			objectReferences, err = GetConfigMapsReferencedBy(parent)
			name = object.Name
		case *corev1.Secret:
			objectReferences, err = GetSecretsReferencedBy(ctx, client, parent)
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
			ok := vss.ContainsOutdatedSecretName(keys, name)
			return ok, nil
		}

		_, ok := objectReferences[name]
		return ok, nil
	}

	namespace := object.GetNamespace()
	result := []reconcile.Request{}

	switch reconcileType {
	case ReconcileForExtendedJob:
		extendedJobs, err := listExtendedJobs(ctx, client, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list ExtendedJobs for ConfigMap reconciles")
		}

		for _, eJob := range extendedJobs.Items {
			if !(eJob.Spec.UpdateOnConfigChange && eJob.IsAutoErrand()) {
				continue
			}
			isRef, err := objReferencedBy(eJob)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      eJob.Name,
						Namespace: eJob.Namespace,
					}})
			}
		}
	default:
		return nil, fmt.Errorf("unkown reconcile type %s", reconcileType.String())
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
			log.Errorf(ctx, "Failed to get ConfigMap '%s': %s", object.Name, err)
			return true
		}

		newResourceVersion = cm.ResourceVersion
	case *corev1.Secret:
		s := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, s)
		if err != nil {
			log.Errorf(ctx, "Failed to get Secret '%s': %s", object.Name, err)
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

func listExtendedJobs(ctx context.Context, client crc.Client, namespace string) (*ejv1.ExtendedJobList, error) {
	log.Debugf(ctx, "Listing ExtendedJobs in namespace '%s'", namespace)
	result := &ejv1.ExtendedJobList{}
	err := client.List(ctx, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ExtendedJobs")
	}

	return result, nil
}
