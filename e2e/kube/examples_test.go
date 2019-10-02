package kube_test

import (
	b64 "encoding/base64"
	"fmt"
	"path"

	"code.cloudfoundry.org/cf-operator/testing"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Examples Directory", func() {
	var (
		example      string
		yamlFilePath string
		kubectl      *testing.Kubectl
	)

	JustBeforeEach(func() {
		kubectl = testing.NewKubectl()
		yamlFilePath = path.Join(examplesDir, example)
		err := testing.Create(namespace, yamlFilePath)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("extended-job auto errand delete example", func() {
		BeforeEach(func() {
			example = "exjob_auto-errand-deletes-pod.yaml"
		})

		It("deletes pod after job is done", func() {
			By("Checking for pods")
			err := kubectl.WaitLabelFilter(namespace, "ready", "pod", fmt.Sprintf("%s=deletes-pod-1", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "terminate", "pod", fmt.Sprintf("%s=deletes-pod-1", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("extended-job auto errand example", func() {
		BeforeEach(func() {
			example = "exjob_auto-errand.yaml"
		})

		It("runs the errand automatically", func() {
			By("Checking for pods")
			err := kubectl.WaitLabelFilter(namespace, "ready", "pod", fmt.Sprintf("%s=one-time-sleep", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=one-time-sleep", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("extended-job auto errand update example", func() {
		BeforeEach(func() {
			example = "exjob_auto-errand-updating.yaml"
		})

		It("triggers job again when config is updated", func() {
			By("Checking for pods")
			err := kubectl.WaitLabelFilter(namespace, "ready", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			By("Delete the pod")
			err = testing.DeleteLabelFilter(namespace, "pod", fmt.Sprintf("%s=auto-errand-sleep-again", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			By("Update the config change")
			yamlFilePath = examplesDir + "exjob_auto-errand-updating_updated.yaml"

			err = testing.Apply(namespace, yamlFilePath)
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("extended-job errand example", func() {
		BeforeEach(func() {
			example = "exjob_errand.yaml"
		})

		It("starts job if trigger is changed manually", func() {
			By("Updating exjob to trigger now")
			yamlFilePath = examplesDir + "exjob_errand_updated.yaml"
			err := testing.Apply(namespace, yamlFilePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pods")
			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", fmt.Sprintf("%s=manual-sleep", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=manual-sleep", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("extended-job output example", func() {
		BeforeEach(func() {
			example = "exjob_output.yaml"
		})

		It("creates a secret from job output", func() {
			By("Checking for pods")
			err := kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=myfoo", ejv1.LabelEJobName))
			Expect(err).ToNot(HaveOccurred())

			By("Checking for secret")
			err = kubectl.WaitForSecret(namespace, "foo-json")
			Expect(err).ToNot(HaveOccurred())

			By("Checking the secret data created")
			outSecret, err := testing.GetData(namespace, "secret", "foo-json", "go-template={{.data.foo}}")
			Expect(err).ToNot(HaveOccurred())
			outSecretDecoded, _ := b64.StdEncoding.DecodeString(string(outSecret))
			Expect(string(outSecretDecoded)).To(Equal("1"))
		})
	})
})
