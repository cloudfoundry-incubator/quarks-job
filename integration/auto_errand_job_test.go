package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("AutoErrandJob", func() {
	AfterEach(func() {
		env.FlushLog()
	})

	Context("when using an AutoErrandJob", func() {
		var (
			qj        qjv1a1.QuarksJob
			tearDowns []machine.TearDownFunc
		)

		BeforeEach(func() {
			qj = env.AutoErrandQuarksJob("autoerrand-job")
		})

		AfterEach(func() {
			Expect(env.TearDownAll(tearDowns)).To(Succeed())
		})

		It("immediately starts the job", func() {
			_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
			Expect(jobs).To(HaveLen(1))
		})

		Context("when the job succeeded", func() {
			It("cleans up job immediately", func() {
				_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
				Expect(err).NotTo(HaveOccurred())
				defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

				jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
				Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
				Expect(jobs).To(HaveLen(1))

				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).ToNot(HaveOccurred())

				By("Checking pod is still there, because delete label is missing")
				Expect(env.PodsDeleted(env.Namespace)).To(BeFalse())
			})

			Context("when pod template has delete label", func() {
				Context("when delete is set to pod", func() {
					BeforeEach(func() {
						qj.Spec.Template.Spec.Template.Labels = map[string]string{"delete": "pod"}
					})

					It("removes job's pod", func() {
						_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
						Expect(err).NotTo(HaveOccurred())
						defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

						jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
						Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
						Expect(jobs).To(HaveLen(1))

						err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
						Expect(err).ToNot(HaveOccurred())

						Expect(env.PodsDeleted(env.Namespace)).To(BeTrue())
					})
				})

				Context("when delete is set to something else", func() {
					BeforeEach(func() {
						qj.Spec.Template.Labels = map[string]string{"delete": "something-else"}
					})

					It("keeps the job's pod", func() {
						_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
						Expect(err).NotTo(HaveOccurred())
						defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

						jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
						Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
						Expect(jobs).To(HaveLen(1))

						err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
						Expect(err).ToNot(HaveOccurred())

						Expect(env.PodsDeleted(env.Namespace)).To(BeFalse())
					})
				})
			})
		})

		Context("when the job failed", func() {
			BeforeEach(func() {
				qj.Spec.Template = env.FailingMultiContainerJobTemplate([]string{"echo", "{}"})
			})

			It("cleans it up when the QuarksJob is deleted", func() {
				_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
				Expect(err).NotTo(HaveOccurred())
				defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

				jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
				Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
				Expect(jobs).To(HaveLen(1))

				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).To(HaveOccurred())

				Expect(tearDown()).To(Succeed())
				err = env.WaitForJobDeletion(env.Namespace, jobs[0].Name)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when configured to update on config change", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ machine.TearDownFunc
			)

			BeforeEach(func() {
				qj.Spec.UpdateOnConfigChange = true
				qj.Spec.Template = env.ConfigJobTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

				tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				tearDown, err = env.CreateSecret(env.Namespace, secret)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				_, tearDownEJ, err = env.CreateQuarksJob(env.Namespace, qj)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDownEJ)

				_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a config content changes", func() {
				It("it creates a new job", func() {
					By("checking if ext job is done")
					qJob, err := env.GetQuarksJob(env.Namespace, qj.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerDone))
					Expect(env.WaitForLogMsg(env.ObservedLogs, "Deleting succeeded job")).ToNot(HaveOccurred())

					By("modifying config")
					c, _ := env.GetConfigMap(env.Namespace, configMap.Name)
					c.Data["fake-key"] = "fake-value"
					_, _, err = env.UpdateConfigMap(env.Namespace, *c)
					Expect(err).NotTo(HaveOccurred())

					By("checking if job is running")
					jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
					Expect(err).NotTo(HaveOccurred())
					Expect(jobs).To(HaveLen(1))
				})
			})
		})

		Context("when enabling update on config change", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ machine.TearDownFunc
			)

			BeforeEach(func() {
				qj.Spec.UpdateOnConfigChange = false
				qj.Spec.Template = env.ConfigJobTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

				tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				tearDown, err = env.CreateSecret(env.Namespace, secret)
				Expect(err).ToNot(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				_, tearDownEJ, err = env.CreateQuarksJob(env.Namespace, qj)
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDownEJ)

				_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when referenced configs are created after the quarks job", func() {
			var (
				configMap  corev1.ConfigMap
				secret     corev1.Secret
				tearDownEJ machine.TearDownFunc
				tearDown   machine.TearDownFunc
			)

			BeforeEach(func() {
				qj.Spec.UpdateOnConfigChange = true
				qj.Spec.Template = env.ConfigJobTemplate()

				configMap = env.DefaultConfigMap("config1")
				secret = env.DefaultSecret("secret1")

			})

			Context("when the quarks job is created after the config map", func() {
				BeforeEach(func() {
					var err error
					tearDown, err = env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())

					_, tearDownEJ, err = env.CreateQuarksJob(env.Namespace, qj)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the config map")
					tearDown, err := env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the quarks job is created after the secret", func() {
				BeforeEach(func() {
					var err error
					tearDown, err = env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())

					_, tearDownEJ, err = env.CreateQuarksJob(env.Namespace, qj)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the secret")
					tearDown, err := env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the quarks job is created after several configs", func() {
				BeforeEach(func() {
					var err error
					_, tearDownEJ, err = env.CreateQuarksJob(env.Namespace, qj)
					Expect(err).NotTo(HaveOccurred())
				})

				It("the job starts", func() {
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDownEJ)

					By("creating the configs")
					tearDown, err := env.CreateSecret(env.Namespace, secret)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					tearDown, err = env.CreateConfigMap(env.Namespace, configMap)
					Expect(err).ToNot(HaveOccurred())
					defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

					By("waiting for the job to start")
					_, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
