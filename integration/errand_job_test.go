package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("ErrandJob", func() {
	jobOwnerRef := func(qJob qjv1a1.QuarksJob) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion:         "quarks.cloudfoundry.org/v1alpha1",
			Kind:               "QuarksJob",
			Name:               qJob.Name,
			UID:                qJob.UID,
			Controller:         pointers.Bool(true),
			BlockOwnerDeletion: pointers.Bool(true),
		}
	}

	var (
		qj        qjv1a1.QuarksJob
		tearDowns []machine.TearDownFunc
	)

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
		env.FlushLog()
	})

	JustBeforeEach(func() {
		_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
	})

	Context("when persisting output", func() {
		BeforeEach(func() {
			qj = env.OutputQuarksJob("quarks")
		})

		It("does persist output", func() {
			jobs, err := env.CollectJobs(env.Namespace, quarksJobLabel, 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarks-job")
			Expect(jobs).To(HaveLen(1))

			for _, name := range []string{"foo-busybox", "fake-nats", "bar-nuts-v1"} {
				secret, err := env.CollectSecret(env.Namespace, name)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Data["fake"]).To(Equal([]byte("value")))
			}
		})
	})

	Context("when trigger is set to now", func() {
		BeforeEach(func() {
			qj = env.ErrandQuarksJob("quarks-job")
		})

		It("starts a job", func() {
			jobs, err := env.CollectJobs(env.Namespace, quarksJobLabel, 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarks-job")
			Expect(jobs).To(HaveLen(1))
		})
	})

	Context("when using manually triggered ErrandJob", func() {
		BeforeEach(func() {
			qj = env.ErrandQuarksJob("quarks-job")
			qj.Spec.Trigger.Strategy = qjv1a1.TriggerManual
		})

		// this test waits 60s for a job not to appear
		It("does not start a job without Run being set to now", func() {
			exists, err := env.WaitForJobExists(env.Namespace, quarksJobLabel)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			latest, err := env.GetQuarksJob(env.Namespace, qj.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerManual))

			err = env.UpdateQuarksJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			exists, err = env.WaitForJobExists(env.Namespace, quarksJobLabel)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	Context("when updating trigger to now", func() {
		BeforeEach(func() {
			qj = env.ErrandQuarksJob("quarks-job")
			qj.Spec.Trigger.Strategy = qjv1a1.TriggerManual
		})

		It("starts a job", func() {
			latest, err := env.GetQuarksJob(env.Namespace, qj.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerManual))

			latest.Spec.Trigger.Strategy = qjv1a1.TriggerNow
			err = env.UpdateQuarksJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			jobs, err := env.CollectJobs(env.Namespace, quarksJobLabel, 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
			Expect(jobs).To(HaveLen(1))
			Expect(jobs[0].GetOwnerReferences()).Should(ContainElement(jobOwnerRef(*latest)))
		})
	})
})
