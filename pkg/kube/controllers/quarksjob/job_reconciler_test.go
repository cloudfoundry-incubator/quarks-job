package quarksjob_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/quarks-job/pkg/kube/controllers/fakes"
	qj "code.cloudfoundry.org/quarks-job/pkg/kube/controllers/quarksjob"
	"code.cloudfoundry.org/quarks-job/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileJob", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
		client     *cfakes.FakeClient
		qJob       *qjv1a1.QuarksJob
		job        *batchv1.Job
		pod1       *corev1.Pod
		env        testing.Catalog
		logs       *observer.ObservedLogs
	)

	BeforeEach(func() {
		err := controllers.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		logs, log = helper.NewTestLogger()

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *qjv1a1.QuarksJob:
				qJob.DeepCopyInto(object)
				return nil
			case *batchv1.Job:
				job.DeepCopyInto(object)
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})
		client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
			switch object := object.(type) {
			case *corev1.PodList:
				list := corev1.PodList{
					Items: []corev1.Pod{*pod1},
				}
				list.DeepCopyInto(object)
			case *corev1.SecretList:
				list := corev1.SecretList{}
				list.DeepCopyInto(object)
			}
			return nil
		})
		manager.GetClientReturns(client)
		client.StatusCalls(func() crc.StatusWriter { return &cfakes.FakeStatusWriter{} })
	})

	JustBeforeEach(func() {
		ctx := ctxlog.NewParentContext(log)
		config := helper.NewConfigWithTimeout(10 * time.Second)
		reconciler, _ = qj.NewJobReconciler(ctx, config, manager)
		qJob, job, pod1 = env.DefaultQuarksJobWithSucceededJob("foo", request.Namespace)
	})

	act := func() (reconcile.Result, error) {
		return reconciler.Reconcile(request)
	}

	Context("and the quarks job does not exist", func() {
		BeforeEach(func() {
			client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "fake-error"))
		})

		It("should log and return, don't requeue", func() {
			result, err := act()
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(logs.FilterMessageSnippet("Failed to find job 'default/foo', not retrying:  \"fake-error\" not found").Len()).To(Equal(1))
		})
	})

	Context("to get the quarks job", func() {
		BeforeEach(func() {
			client.GetReturns(fmt.Errorf("fake-error"))
		})

		It("should log and return, requeue", func() {
			_, err := act()
			Expect(err).To(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Failed to get job 'default/foo': fake-error").Len()).To(Equal(1))
		})
	})

	Context("With a succeeded Job", func() {
		It("deletes the job immediately", func() {
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(1))
			Expect(client.StatusCallCount()).To(Equal(1))
		})

		It("deletes owned pod together with the job", func() {
			if job.Spec.Template.ObjectMeta.Labels == nil {
				job.Spec.Template.ObjectMeta.Labels = map[string]string{}
			}
			job.Spec.Template.ObjectMeta.Labels["delete"] = qj.DeleteKind

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(2))
			Expect(client.StatusCallCount()).To(Equal(1))
		})

		It("deletes latest owned pod together with the job", func() {
			var latestTimestamp metav1.Time
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *corev1.PodList:
					pod2 := *pod1
					latestTimestamp = metav1.Now()
					pod2.SetCreationTimestamp(latestTimestamp)
					list := corev1.PodList{
						Items: []corev1.Pod{*pod1, pod2},
					}
					list.DeepCopyInto(object)
				case *corev1.SecretList:
					list := corev1.SecretList{}
					list.DeepCopyInto(object)
				}
				return nil
			})
			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch pod := object.(type) {
				case *corev1.Pod:
					Expect(pod.GetCreationTimestamp()).To(Equal(latestTimestamp))
					return nil
				}
				return nil
			})

			if job.Spec.Template.ObjectMeta.Labels == nil {
				job.Spec.Template.ObjectMeta.Labels = map[string]string{}
			}
			job.Spec.Template.ObjectMeta.Labels["delete"] = qj.DeleteKind

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(2))
			Expect(client.StatusCallCount()).To(Equal(1))
		})

		It("handles an error when getting job's quarks job reference failed", func() {
			job.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not find parent quarksJob reference for Job"))
		})

		It("handles an error when getting job's quarks job parent reference failed", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *batchv1.Job:
					job.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("getting parent quarksJob in Job Reconciler for job"))
		})

		It("handles an error when deleting job failed", func() {
			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch object := object.(type) {
				case *batchv1.Job:
					Expect(object.GetName()).To(Equal("foo-job"))
					return fmt.Errorf("fake-error")
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Cannot delete succeeded job").Len()).To(Equal(1))
		})

		It("handles an error when deleting pod failed", func() {
			if job.Spec.Template.ObjectMeta.Labels == nil {
				job.Spec.Template.ObjectMeta.Labels = map[string]string{}
			}
			job.Spec.Template.ObjectMeta.Labels["delete"] = qj.DeleteKind

			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch object.(type) {
				case *corev1.Pod:
					return fmt.Errorf("fake-error")
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Cannot delete succeeded job's pod").Len()).To(Equal(1))
		})

		It("handles an error when listing pod failed", func() {
			if job.Spec.Template.ObjectMeta.Labels == nil {
				job.Spec.Template.ObjectMeta.Labels = map[string]string{}
			}
			job.Spec.Template.ObjectMeta.Labels["delete"] = qj.DeleteKind

			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *corev1.PodList:
					return fmt.Errorf("fake-error")
				case *corev1.SecretList:
					list := corev1.SecretList{}
					list.DeepCopyInto(object)
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Cannot find job's pod").Len()).To(Equal(1))
			Expect(logs.FilterMessageSnippet(fmt.Sprintf("Listing job's '%s/%s' pods failed.", job.Namespace, job.Name)).Len()).To(Equal(1))
		})

		It("handles an error when pod list is empty", func() {
			if job.Spec.Template.ObjectMeta.Labels == nil {
				job.Spec.Template.ObjectMeta.Labels = map[string]string{}
			}
			job.Spec.Template.ObjectMeta.Labels["delete"] = qj.DeleteKind

			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *corev1.PodList:
					list := corev1.PodList{}
					list.DeepCopyInto(object)
				case *corev1.SecretList:
					list := corev1.SecretList{}
					list.DeepCopyInto(object)
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Cannot find job's pod").Len()).To(Equal(1))
			Expect(logs.FilterMessageSnippet(fmt.Sprintf("Job '%s/%s' does not own any pods?", job.Namespace, job.Name)).Len()).To(Equal(1))
		})
	})

	Context("With a failed Job", func() {
		JustBeforeEach(func() {
			job.Status.Succeeded = 0
			job.Status.Failed = 1
		})

		It("does not delete the job immediately", func() {
			_, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

	})
})
