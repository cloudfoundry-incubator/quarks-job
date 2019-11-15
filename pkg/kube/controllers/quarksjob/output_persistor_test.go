package quarksjob_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientfake "k8s.io/client-go/kubernetes/fake"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	clientsetfake "code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned/fake"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/quarksjob"
	"code.cloudfoundry.org/quarks-job/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

var _ = Describe("OutputPersistor", func() {
	var (
		namespace          string
		qJob               *qjv1a1.QuarksJob
		pod                *corev1.Pod
		env                testing.Catalog
		clientSet          *clientfake.Clientset
		versionedClientSet *clientsetfake.Clientset
		po                 *quarksjob.OutputPersistor
	)

	BeforeEach(func() {
		namespace = "test"
		qJob, _, pod = env.DefaultQuarksJobWithSucceededJob("foo")
		clientSet = clientfake.NewSimpleClientset()
		versionedClientSet = clientsetfake.NewSimpleClientset()
		po = quarksjob.NewOutputPersistor(namespace, pod.Name, clientSet, versionedClientSet, "/tmp/")
	})

	JustBeforeEach(func() {
		// Create necessary kube resources
		_, err := versionedClientSet.QuarksjobV1alpha1().QuarksJobs(namespace).Create(qJob)
		Expect(err).NotTo(HaveOccurred())
		_, err = clientSet.CoreV1().Pods(namespace).Create(pod)
		Expect(err).NotTo(HaveOccurred())
		// Create output file
		err = os.MkdirAll("/tmp/busybox", os.ModePerm)
		Expect(err).NotTo(HaveOccurred())
		dataJson, err := json.Marshal(map[string]string{
			"hello": "world",
		})
		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile("/tmp/busybox/output.json", dataJson, 0755)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("With a succeeded Job", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "busybox",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			}
		})

		Context("when output persistence is not configured", func() {
			BeforeEach(func() {
				qJob.Spec.Output = nil
			})

			It("does not persist output", func() {
				err := po.Persist()
				Expect(err).NotTo(HaveOccurred())
				_, err = clientSet.CoreV1().Secrets(namespace).Get("foo-busybox", metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when output persistence is configured", func() {
			BeforeEach(func() {
				qJob.Spec.Output = &qjv1a1.Output{
					NamePrefix: "foo-",
					SecretLabels: map[string]string{
						"key": "value",
					},
				}
			})

			It("creates the secret and persists the output and have the configured labels", func() {
				err := po.Persist()
				Expect(err).NotTo(HaveOccurred())
				secret, _ := clientSet.CoreV1().Secrets(namespace).Get("foo-busybox", metav1.GetOptions{})
				Expect(secret).ShouldNot(BeNil())
				Expect(secret.Labels).Should(Equal(map[string]string{
					"quarks.cloudfoundry.org/container-name": "busybox",
					"key":                                    "value"}))
			})
		})

		Context("when versioned output is enabled", func() {
			BeforeEach(func() {
				qJob.Spec.Output = &qjv1a1.Output{
					NamePrefix: "foo-",
					SecretLabels: map[string]string{
						"key":        "value",
						"fake-label": "fake-deployment",
					},
					Versioned: true,
				}
			})

			It("creates versioned manifest secret and persists the output", func() {
				err := po.Persist()
				Expect(err).NotTo(HaveOccurred())
				secret, _ := clientSet.CoreV1().Secrets(namespace).Get("foo-busybox-v1", metav1.GetOptions{})
				Expect(secret).ShouldNot(BeNil())
				Expect(secret.Labels).Should(Equal(map[string]string{
					"quarks.cloudfoundry.org/container-name": "busybox",
					"fake-label":                             "fake-deployment",
					versionedsecretstore.LabelSecretKind:     "versionedSecret",
					versionedsecretstore.LabelVersion:        "1",
					"key":                                    "value"}))
			})
		})
	})

	Context("With a failed Job", func() {
		BeforeEach(func() {
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{
				{
					Name: "busybox",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
						},
					},
				},
			}
		})

		Context("when WriteOnFailure is set", func() {
			BeforeEach(func() {
				qJob.Spec.Output = &qjv1a1.Output{
					NamePrefix:     "foo-",
					WriteOnFailure: true,
				}
			})

			It("does persist the output", func() {
				err := po.Persist()
				Expect(err).NotTo(HaveOccurred())
				_, err = clientSet.CoreV1().Secrets(namespace).Get("foo-busybox", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
