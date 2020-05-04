package quarksjob_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

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
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/fakes"
	. "code.cloudfoundry.org/quarks-job/pkg/kube/controllers/quarksjob"
	"code.cloudfoundry.org/quarks-job/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ErrandReconciler", func() {
	Describe("Reconcile", func() {
		var (
			env                        testing.Catalog
			logs                       *observer.ObservedLogs
			log                        *zap.SugaredLogger
			mgr                        *fakes.FakeManager
			request                    reconcile.Request
			reconciler                 reconcile.Reconciler
			qJob                       qjv1a1.QuarksJob
			serviceAccount             corev1.ServiceAccount
			setOwnerReferenceCallCount int
		)

		namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", Labels: map[string]string{qjv1a1.LabelServiceAccount: "persist-output"}}}

		newRequest := func(qJob qjv1a1.QuarksJob) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      qJob.Name,
					Namespace: qJob.Namespace,
				},
			}
		}

		clientGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			switch obj := obj.(type) {
			case *corev1.Namespace:
				namespace.DeepCopyInto(obj)
				return nil
			case *qjv1a1.QuarksJob:
				qJob.DeepCopyInto(obj)
				return nil
			case *corev1.ServiceAccount:
				serviceAccount.DeepCopyInto(obj)
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		}

		setOwnerReference := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
			setOwnerReferenceCallCount++
			return nil
		}

		JustBeforeEach(func() {
			ctx := ctxlog.NewParentContext(log)
			config := helper.NewConfigWithTimeout(10 * time.Second)
			reconciler = NewErrandReconciler(
				ctx,
				config,
				mgr,
				setOwnerReference,
				vss.NewVersionedSecretStore(mgr.GetClient()),
			)
		})

		act := func() (reconcile.Result, error) {
			return reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			mgr = &fakes.FakeManager{}
			setOwnerReferenceCallCount = 0
			logs, log = helper.NewTestLogger()
		})

		Context("when client fails", func() {
			var (
				client   fakes.FakeClient
				qJobName string
			)

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				qJobName = "fake-qj"
				qJob = env.ErrandQuarksJob(qJobName, qJob.Namespace)
				serviceAccount = env.DefaultServiceAccount("persist-output-service-account", qJob.Namespace)
				client.GetCalls(clientGetStub)
				request = newRequest(qJob)
			})

			Context("and the quarks job does not exist", func() {
				BeforeEach(func() {
					client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "fake-error"))
				})

				It("should log and return, don't requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Failed to find quarks job '/%s', not retrying:  \"fake-error\" not found", qJobName)).Len()).To(Equal(1))
				})
			})

			Context("to get the quarks job", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return, requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Failed to get quarks job '/%s': fake-error", qJobName)).Len()).To(Equal(1))
				})
			})

			Context("when client fails to update quarks job", func() {
				BeforeEach(func() {
					client.UpdateReturns(fmt.Errorf("fake-error"))
				})

				It("should return and try to requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Failed to revert to 'trigger.strategy=manual' on job '/%s': fake-error", qJobName)).Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(0))
				})
			})

			Context("when client fails to create jobs", func() {
				BeforeEach(func() {
					client.CreateReturns(fmt.Errorf("fake-error"))
				})

				It("should log create error and requeue", func() {
					_, err := act()
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Failed to create job '/%s': fake-error", qJobName)).Len()).To(Equal(1))
					Expect(err).To(HaveOccurred())
					Expect(client.CreateCallCount()).To(Equal(1))
				})
			})

			Context("when client fails to create jobs because it already exists", func() {
				BeforeEach(func() {
					client.UpdateReturns(nil)
					client.CreateReturns(apierrors.NewAlreadyExists(schema.GroupResource{}, "fake-error"))
					client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })
				})

				It("should log skip message and not requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Skip '/%s': already running", qJobName)).Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(1))
				})
			})

		})

		Context("when quarks job is reconciled", func() {
			var (
				client       fakes.FakeClient
				statusWriter fakes.FakeStatusWriter
			)

			Context("and the errand is a manual errand", func() {
				BeforeEach(func() {
					qJob = env.ErrandQuarksJob("fake-qj", qJob.Namespace)
					serviceAccount = env.DefaultServiceAccount("persist-output-service-account", qJob.Namespace)
					client = fakes.FakeClient{}
					mgr.GetClientReturns(&client)
					client.GetCalls(clientGetStub)
					client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })

					request = newRequest(qJob)
				})

				It("should set run back and create a job", func() {
					Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerNow))

					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerManual))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})
			})

			Context("and the errand is an auto-errand", func() {
				BeforeEach(func() {
					qJob = env.AutoErrandQuarksJob("fake-qj")
					serviceAccount = env.DefaultServiceAccount("persist-output-service-account", qJob.Namespace)
					client = fakes.FakeClient{}
					statusWriter = fakes.FakeStatusWriter{}
					mgr.GetClientReturns(&client)
					client.GetCalls(clientGetStub)
					client.StatusCalls(func() crc.StatusWriter { return &statusWriter })

					request = newRequest(qJob)
				})

				It("should set the trigger strategy to done and immediately trigger the job", func() {
					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerDone))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should requeue reconcile when quarks job is in meltdown", func() {
					now := metav1.Now()
					qJob.Status.LastReconcile = &now

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RequeueAfter).To(Equal(config.MeltdownRequeueAfter))
				})

				It("handles an error when updating job's strategy failed", func() {
					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerDone))
								return fmt.Errorf("fake-error")
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet("Failed to traverse to 'trigger.strategy=done' on job").Len()).To(Equal(1))
				})

				It("handles an error when updating job's reconcile timestamp failed", func() {
					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerDone))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					statusCallQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								if qJob.Status.LastReconcile != nil {
									return fmt.Errorf("fake-error")
								}
							}
							return nil
						},
					)
					statusWriter.UpdateCalls(statusCallQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet("Failed to update reconcile timestamp on job").Len()).To(Equal(1))
				})
			})

			Context("and the auto-errand is updated on config change", func() {
				var (
					configMap *corev1.ConfigMap
					secret    *corev1.Secret
					qJobName  string
				)

				BeforeEach(func() {
					c1 := env.DefaultConfigMap("config1", qJob.Namespace)
					configMap = &c1
					s1 := env.DefaultSecret("secret1", qJob.Namespace)
					secret = &s1

					serviceAccount = env.DefaultServiceAccount("persist-output-service-account", qJob.Namespace)
					qJobName = "fake-qj"
					qJob = env.AutoErrandQuarksJob(qJobName)
					qJob.Spec.Template = env.ConfigJobTemplate()
					qJob.Spec.UpdateOnConfigChange = true
					qJob.Spec.Trigger.Strategy = qjv1a1.TriggerOnce
					client = fakes.FakeClient{}
					mgr.GetClientReturns(&client)
					client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })

					request = newRequest(qJob)
				})

				It("should trigger the job", func() {
					client.GetCalls(func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
						switch obj := obj.(type) {
						case *corev1.Namespace:
							namespace.DeepCopyInto(obj)
							return nil
						case *qjv1a1.QuarksJob:
							qJob.DeepCopyInto(obj)
							return nil
						case *corev1.ConfigMap:
							configMap.DeepCopyInto(obj)
							return nil
						case *corev1.Secret:
							secret.DeepCopyInto(obj)
							return nil
						case *corev1.ServiceAccount:
							serviceAccount.DeepCopyInto(obj)
							return nil
						}
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					})

					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch qJob := object.(type) {
							case *qjv1a1.QuarksJob:
								Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerDone))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should skip when references are missing", func() {
					client.GetCalls(func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
						switch obj := obj.(type) {
						case *corev1.Namespace:
							namespace.DeepCopyInto(obj)
							return nil
						case *qjv1a1.QuarksJob:
							qJob.DeepCopyInto(obj)
							return nil
						case *corev1.ServiceAccount:
							serviceAccount.DeepCopyInto(obj)
							return nil
						}
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					})

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeTrue())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Skip create job '/%s' due to configMap 'config1' not found", qJobName)).Len()).To(Equal(1))

					client.GetCalls(func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
						switch obj := obj.(type) {
						case *corev1.Namespace:
							namespace.DeepCopyInto(obj)
							return nil
						case *qjv1a1.QuarksJob:
							qJob.DeepCopyInto(obj)
							return nil
						case *corev1.ConfigMap:
							configMap.DeepCopyInto(obj)
							return nil
						case *corev1.ServiceAccount:
							serviceAccount.DeepCopyInto(obj)
							return nil
						}
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					})

					result, err = act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeTrue())
					Expect(logs.FilterMessageSnippet(fmt.Sprintf("Skip create job '/%s' due to secret 'secret1' not found", qJobName)).Len()).To(Equal(1))
				})
			})
		})
	})
})
