package quarksjob

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

func newNSPredicate(ctx context.Context, client client.Client, id string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			name := e.Meta.GetNamespace()
			ns := &corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
				ctxlog.Errorf(ctx, "failed to get namespaces '%s'", name)
				return false
			}
			return qjv1a1.IsMonitoredNamespace(ns, id)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			name := e.MetaNew.GetNamespace()
			ns := &corev1.Namespace{}
			if err := client.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
				ctxlog.Errorf(ctx, "failed to get namespaces '%s'", name)
				return false
			}
			return qjv1a1.IsMonitoredNamespace(ns, id)
		},
	}
}
