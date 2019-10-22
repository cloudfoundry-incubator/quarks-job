package extendedjob_test

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

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers"
	. "code.cloudfoundry.org/quarks-job/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/fakes"
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
			eJob                       ejv1.ExtendedJob
			serviceAccount             corev1.ServiceAccount
			setOwnerReferenceCallCount int
		)

		newRequest := func(eJob ejv1.ExtendedJob) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      eJob.Name,
					Namespace: eJob.Namespace,
				},
			}
		}

		clientGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			switch obj := obj.(type) {
			case *ejv1.ExtendedJob:
				eJob.DeepCopyInto(obj)
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
			config := &config.Config{
				CtxTimeOut:           10 * time.Second,
				MeltdownDuration:     config.MeltdownDuration,
				MeltdownRequeueAfter: config.MeltdownRequeueAfter,
			}
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
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				eJob = env.ErrandExtendedJob("fake-ejob")
				serviceAccount = env.DefaultServiceAccount("persist-output-service-account")
				client.GetCalls(clientGetStub)
				request = newRequest(eJob)
			})

			Context("and the extended job does not exist", func() {
				BeforeEach(func() {
					client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "fake-error"))
				})

				It("should log and return, don't requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet("Failed to find extended job '/fake-ejob', not retrying:  \"fake-error\" not found").Len()).To(Equal(1))
				})
			})

			Context("to get the extended job", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return, requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to get extended job '/fake-ejob': fake-error").Len()).To(Equal(1))
				})
			})

			Context("when client fails to update extended job", func() {
				BeforeEach(func() {
					client.UpdateReturns(fmt.Errorf("fake-error"))
				})

				It("should return and try to requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to revert to 'trigger.strategy=manual' on job 'fake-ejob': fake-error").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(0))
				})
			})

			Context("when client fails to create jobs", func() {
				BeforeEach(func() {
					client.CreateReturns(fmt.Errorf("fake-error"))
				})

				It("should log create error and requeue", func() {
					_, err := act()
					Expect(logs.FilterMessageSnippet("Failed to create job 'fake-ejob': could not create service account for " +
						"pod in eJob fake-ejob: fake-error").Len()).To(Equal(1))
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
					Expect(logs.FilterMessageSnippet("Skip 'fake-ejob': already running").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(3))
				})
			})

		})

		Context("when extended job is reconciled", func() {
			var client fakes.FakeClient
			var statusWriter fakes.FakeStatusWriter

			Context("and the errand is a manual errand", func() {
				BeforeEach(func() {
					eJob = env.ErrandExtendedJob("fake-pod")
					serviceAccount = env.DefaultServiceAccount("persist-output-service-account")
					client = fakes.FakeClient{}
					mgr.GetClientReturns(&client)
					client.GetCalls(clientGetStub)
					client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })

					request = newRequest(eJob)
				})

				It("should set run back and create a job", func() {
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerNow))

					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerManual))
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
					eJob = env.AutoErrandExtendedJob("fake-pod")
					serviceAccount = env.DefaultServiceAccount("persist-output-service-account")
					client = fakes.FakeClient{}
					statusWriter = fakes.FakeStatusWriter{}
					mgr.GetClientReturns(&client)
					client.GetCalls(clientGetStub)
					client.StatusCalls(func() crc.StatusWriter { return &statusWriter })

					request = newRequest(eJob)
				})

				It("should set the trigger strategy to done and immediately trigger the job", func() {
					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
				})

				It("should requeue reconcile when extended job is in meltdown", func() {
					now := metav1.Now()
					eJob.Status.LastReconcile = &now

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RequeueAfter).To(Equal(config.MeltdownRequeueAfter))
				})

				It("handles an error when updating job's strategy failed", func() {
					callQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))
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
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))
							}
							return nil
						},
					)
					client.UpdateCalls(callQueue.Calls)

					statusCallQueue := helper.NewCallQueue(
						func(context context.Context, object runtime.Object) error {
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								if eJob.Status.LastReconcile != nil {
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
				)

				BeforeEach(func() {
					c1 := env.DefaultConfigMap("config1")
					configMap = &c1
					s1 := env.DefaultSecret("secret1")
					secret = &s1

					serviceAccount = env.DefaultServiceAccount("persist-output-service-account")
					eJob = env.AutoErrandExtendedJob("fake-pod")
					eJob.Spec.Template = env.ConfigJobTemplate()
					eJob.Spec.UpdateOnConfigChange = true
					eJob.Spec.Trigger.Strategy = ejv1.TriggerOnce
					client = fakes.FakeClient{}
					mgr.GetClientReturns(&client)
					client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })

					request = newRequest(eJob)
				})

				It("should trigger the job", func() {
					client.GetCalls(func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
						switch obj := obj.(type) {
						case *ejv1.ExtendedJob:
							eJob.DeepCopyInto(obj)
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
							switch eJob := object.(type) {
							case *ejv1.ExtendedJob:
								Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))
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
						case *ejv1.ExtendedJob:
							eJob.DeepCopyInto(obj)
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
					Expect(logs.FilterMessageSnippet("Skip create job 'fake-pod' due to configMap 'config1' not found").Len()).To(Equal(1))

					client.GetCalls(func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
						switch obj := obj.(type) {
						case *ejv1.ExtendedJob:
							eJob.DeepCopyInto(obj)
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
					Expect(logs.FilterMessageSnippet("Skip create job 'fake-pod' due to secret 'secret1' not found").Len()).To(Equal(1))
				})
			})
		})
	})
})
