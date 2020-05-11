package quarksjob

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	qjv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/fakes"
)

var _ = Describe("nsPredicate", func() {
	var (
		client      *fakes.FakeClient
		namespace   corev1.Namespace
		nsPredicate predicate.Funcs
		e           event.CreateEvent
	)

	BeforeEach(func() {
		client = &fakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *corev1.Namespace:
				namespace.DeepCopyInto(object)
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})
	})

	Context("when namespace is setup correctly", func() {
		JustBeforeEach(func() {
			namespace = corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1234",
					Labels: map[string]string{
						qjv1.LabelNamespace:      "1234",
						qjv1.LabelServiceAccount: "default",
					},
				},
			}
			e = event.CreateEvent{Meta: &namespace.ObjectMeta}
		})

		Context("when monitor id is different", func() {
			JustBeforeEach(func() {
				nsPredicate = newNSPredicate(context.TODO(), client, "abcde")
			})

			It("returns false", func() {
				Expect(nsPredicate.Create(e)).To(BeFalse())
			})
		})

		Context("when monitor id is equal", func() {
			JustBeforeEach(func() {
				nsPredicate = newNSPredicate(context.TODO(), client, "1234")
			})

			It("returns true", func() {
				Expect(nsPredicate.Create(e)).To(BeTrue())
			})
		})
	})

	Context("when namespace has no labels", func() {
		JustBeforeEach(func() {
			namespace = corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "n1234",
				},
			}
			e = event.CreateEvent{Meta: &namespace.ObjectMeta}
		})

		Context("when monitor id is different", func() {
			JustBeforeEach(func() {
				nsPredicate = newNSPredicate(context.TODO(), client, "abcde")
			})

			It("returns false", func() {
				Expect(nsPredicate.Create(e)).To(BeFalse())
			})
		})

		Context("when monitor id is equal", func() {
			JustBeforeEach(func() {
				nsPredicate = newNSPredicate(context.TODO(), client, "1234")
			})

			It("returns false", func() {
				Expect(nsPredicate.Create(e)).To(BeFalse())
			})
		})
	})
})
