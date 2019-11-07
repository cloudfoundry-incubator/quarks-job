package kube_test

import (
	b64 "encoding/base64"
	"fmt"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing"
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

	Context("quarks job auto errand delete example", func() {
		BeforeEach(func() {
			example = "qjob_auto-errand-deletes-pod.yaml"
		})

		It("deletes pod after job is done", func() {
			By("Checking for pods")
			err := kubectl.WaitForPod(namespace, fmt.Sprintf("%s=deletes-pod-1", qjv1a1.LabelQJobName), "deletes-pod-1")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "terminate", "pod", fmt.Sprintf("%s=deletes-pod-1", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("quarks job auto errand example", func() {
		BeforeEach(func() {
			example = "qjob_auto-errand.yaml"
		})

		It("runs the errand automatically", func() {
			By("Checking for pods")
			err := kubectl.WaitForPod(namespace, fmt.Sprintf("%s=one-time-sleep", qjv1a1.LabelQJobName), "one-time-sleep")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=one-time-sleep", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("quarks job auto errand update example", func() {
		BeforeEach(func() {
			example = "qjob_auto-errand-updating.yaml"
		})

		It("triggers job again when config is updated", func() {
			By("Checking for pods")

			err := kubectl.WaitForPod(namespace, fmt.Sprintf("%s=auto-errand-sleep-again", qjv1a1.LabelQJobName), "auto-errand-sleep-again")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())

			By("Delete the pod")
			err = testing.DeleteLabelFilter(namespace, "pod", fmt.Sprintf("%s=auto-errand-sleep-again", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())

			By("Update the config change")
			yamlFilePath = examplesDir + "qjob_auto-errand-updating_updated.yaml"

			err = testing.Apply(namespace, yamlFilePath)
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPod(namespace, fmt.Sprintf("%s=auto-errand-sleep-again", qjv1a1.LabelQJobName), "auto-errand-sleep-again")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=auto-errand-sleep-again", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("quarks job errand example", func() {
		BeforeEach(func() {
			example = "qjob_errand.yaml"
		})

		It("starts job if trigger is changed manually", func() {
			By("Updating qjob to trigger now")
			yamlFilePath = examplesDir + "qjob_errand_updated.yaml"
			err := testing.Apply(namespace, yamlFilePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pods")
			err = kubectl.WaitForPod(namespace, fmt.Sprintf("%s=manual-sleep", qjv1a1.LabelQJobName), "manual-sleep")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=manual-sleep", qjv1a1.LabelQJobName))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("quarks job output example", func() {
		BeforeEach(func() {
			example = "qjob_output.yaml"
		})

		It("creates a secret from job output", func() {
			By("Checking for pods")
			err := kubectl.WaitLabelFilter(namespace, "complete", "pod", fmt.Sprintf("%s=myfoo", qjv1a1.LabelQJobName))
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
