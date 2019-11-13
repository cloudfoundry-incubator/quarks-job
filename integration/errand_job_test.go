package integration_test

import (
	"fmt"

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

	AfterEach(func() {
		env.FlushLog()
	})

	Context("when using manually triggered ErrandJob", func() {
		It("does not start a job without Run being set to now", func() {
			qj := env.ErrandQuarksJob("quarks-job")
			qj.Spec.Trigger.Strategy = qjv1a1.TriggerManual
			_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			exists, err := env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())

			latest, err := env.GetQuarksJob(env.Namespace, qj.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerManual))

			err = env.UpdateQuarksJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			exists, err = env.WaitForJobExists(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("starts a job when creating quarks job with now", func() {
			qj := env.ErrandQuarksJob("quarks-job")
			_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarks-job")
			Expect(jobs).To(HaveLen(1))
		})

		It("starts a job when updating quarks job to now", func() {
			qj := env.ErrandQuarksJob("quarks-job")
			qj.Spec.Trigger.Strategy = qjv1a1.TriggerManual
			_, tearDown, err := env.CreateQuarksJob(env.Namespace, qj)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			latest, err := env.GetQuarksJob(env.Namespace, qj.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(latest.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerManual))

			latest.Spec.Trigger.Strategy = qjv1a1.TriggerNow
			err = env.UpdateQuarksJob(env.Namespace, *latest)
			Expect(err).NotTo(HaveOccurred())

			jobs, err := env.CollectJobs(env.Namespace, fmt.Sprintf("%s=true", qjv1a1.LabelQuarksJob), 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for jobs from quarksJob")
			Expect(jobs).To(HaveLen(1))

			Expect(jobs[0].GetOwnerReferences()).Should(ContainElement(jobOwnerRef(*latest)))
		})
	})
})
