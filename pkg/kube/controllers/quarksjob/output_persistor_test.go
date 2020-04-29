package quarksjob_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

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
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("OutputPersistor", func() {
	var dataJSON = []byte("{\"hello\": \"world\"}")

	var (
		namespace          string
		qJob               *qjv1a1.QuarksJob
		pod                *corev1.Pod
		env                testing.Catalog
		clientSet          *clientfake.Clientset
		versionedClientSet *clientsetfake.Clientset
		po                 *quarksjob.OutputPersistor
		tmpDir             string
	)

	BeforeEach(func() {
		namespace = "test"
		qJob, _, pod = env.DefaultQuarksJobWithSucceededJob("foo", namespace)
		clientSet = clientfake.NewSimpleClientset()
		versionedClientSet = clientsetfake.NewSimpleClientset()
		_, log := helper.NewTestLogger()

		var err error
		tmpDir, err = ioutil.TempDir("/tmp", "quarks-job-unit")
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(tmpDir, "busybox"), 0755)).ToNot(HaveOccurred())

		po = quarksjob.NewOutputPersistor(log, namespace, pod.Name, clientSet, versionedClientSet, tmpDir)
	})

	JustBeforeEach(func() {
		// Create necessary kube resources
		_, err := versionedClientSet.QuarksjobV1alpha1().QuarksJobs(namespace).Create(context.Background(), qJob, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = clientSet.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when persisting one output", func() {
		JustBeforeEach(func() {
			// Create output file
			err := ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "output.json"), dataJSON, 0755)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With a succeeded Job", func() {
			BeforeEach(func() {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "busybox",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
						},
					},
				}
			})

			Context("when output persistence is not configured", func() {
				BeforeEach(func() {
					qJob.Spec.Output = nil
				})

				It("does not persist output", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when output persistence is configured", func() {
				BeforeEach(func() {
					additionalLabels := map[string]string{
						"quarks.cloudfoundry.org/entanglement": "foo-busybox",
					}
					qJob.Spec.Output = &qjv1a1.Output{
						SecretLabels: map[string]string{
							"key": "value",
						},
						OutputMap: qjv1a1.OutputMap{
							"busybox": qjv1a1.NewFileToSecret("output.json", "foo-busybox", false, additionalLabels),
						},
					}
				})

				It("creates the secret and persists the output and have the configured labels", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					secret, _ := clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(secret).ShouldNot(BeNil())
					Expect(secret.Labels).Should(Equal(map[string]string{
						"quarks.cloudfoundry.org/container-name": "busybox",
						"quarks.cloudfoundry.org/entanglement":   "foo-busybox",
						"key":                                    "value"}))
				})

				Context("when the output file is not json valid", func() {
					BeforeEach(func() {
						// Create faulty output file
						faultydataJSON := []byte("{\"hello\"= \"world\"}")
						err := ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "faultyoutput.json"), faultydataJSON, 0755)
						Expect(err).NotTo(HaveOccurred())

						qJob.Spec.Output.OutputMap = qjv1a1.OutputMap{
							"busybox": qjv1a1.NewFileToSecret("faultyoutput.json", "foo-busybox", false, map[string]string{}),
						}
					})

					It("should throw out an error", func() {
						err := po.Persist(context.Background())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("failed to convert output file"))
					})
				})
			})

			Context("when versioned output is enabled", func() {
				additionalLabels := map[string]string{
					"quarks.cloudfoundry.org/entanglement": "foo-busybox",
				}
				BeforeEach(func() {
					qJob.Spec.Output = &qjv1a1.Output{
						SecretLabels: map[string]string{
							"key":        "value",
							"fake-label": "fake-deployment",
						},
						OutputMap: qjv1a1.OutputMap{
							"busybox": qjv1a1.FilesToSecrets{
								"output.json": qjv1a1.SecretOptions{
									Name:                   "foo-busybox",
									Versioned:              true,
									AdditionalSecretLabels: additionalLabels,
								},
							},
						},
					}
				})

				It("creates versioned manifest secret and persists the output", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					secret, _ := clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox-v1", metav1.GetOptions{})
					Expect(secret).ShouldNot(BeNil())
					Expect(secret.Labels).Should(Equal(map[string]string{
						"quarks.cloudfoundry.org/entanglement":   "foo-busybox",
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
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
						},
					},
				}
			})

			Context("when WriteOnFailure is set", func() {
				BeforeEach(func() {
					qJob.Spec.Output = &qjv1a1.Output{
						WriteOnFailure: true,
						OutputMap: qjv1a1.OutputMap{
							"busybox": qjv1a1.FilesToSecrets{
								"output.json": qjv1a1.SecretOptions{
									Name: "foo-busybox",
								},
							},
						},
					}
				})

				It("does persist the output", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).ToNot(HaveOccurred())
		})
	})

	Context("when persisting multiple outputs", func() {
		JustBeforeEach(func() {
			err := ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "output.json"), dataJSON, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "output-nats.json"), dataJSON, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "output-nuts.json"), dataJSON, 0755)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("With a succeeded Job", func() {
			BeforeEach(func() {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "busybox",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
						},
					},
				}
			})

			Context("when output persistence is not configured", func() {
				BeforeEach(func() {
					qJob.Spec.Output = nil
				})

				It("does not persist output", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "nats", metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "nuts", metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when output persistence is configured", func() {
				BeforeEach(func() {
					qJob.Spec.Output = &qjv1a1.Output{
						OutputMap: env.DefaultOutputMap(),
					}
				})

				It("creates the secret and persists the output and have the configured labels", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					secret, _ := clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(secret).ShouldNot(BeNil())
					secret, _ = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "fake-nats", metav1.GetOptions{})
					Expect(secret).ShouldNot(BeNil())
					secret, _ = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "bar-nuts-v1", metav1.GetOptions{})
					Expect(secret).ShouldNot(BeNil())
				})
			})

			Context("when output persistence with fan out is configured", func() {
				provideContent := func(data map[string]map[string]string) []byte {
					tmp := map[string]string{}
					for k, v := range data {
						valueBytes, err := json.Marshal(v)
						Expect(err).ToNot(HaveOccurred())

						tmp[k] = string(valueBytes)
					}

					bytes, err := json.Marshal(tmp)
					Expect(err).ToNot(HaveOccurred())

					return bytes
				}

				BeforeEach(func() {
					qJob.Spec.Output = &qjv1a1.Output{
						OutputMap: qjv1a1.OutputMap{
							"busybox": qjv1a1.NewFileToSecrets("provides.json", "link-nats-deployment", false, map[string]string{}),
						},
					}

					Expect(ioutil.WriteFile(
						filepath.Join(tmpDir, "busybox", "provides.json"),
						[]byte(provideContent(map[string]map[string]string{
							"nats-nats": {
								"nats.user":     "admin",
								"nats.password": "changeme",
								"nats.port":     "1337",
							},
							"nats-nuts": {
								"nats.user":     "udmin",
								"nats.password": "chungeme",
								"nats.port":     "1337",
							},
						})), 0640)).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					qJob.Spec.Output = nil
					Expect(os.RemoveAll(tmpDir)).ToNot(HaveOccurred())
				})

				It("creates a secret per each key/value of the given input file", func() {
					Expect(po.Persist(context.Background())).NotTo(HaveOccurred())

					Expect(clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "link-nats-deployment-nats-nats", metav1.GetOptions{})).ShouldNot(BeNil())
					Expect(clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "link-nats-deployment-nats-nuts", metav1.GetOptions{})).ShouldNot(BeNil())
				})
			})
		})

		Context("With a failed Job", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(tmpDir, "busybox", "output.json"), dataJSON, 0755)
				Expect(err).NotTo(HaveOccurred())

				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "busybox",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
						},
					},
				}
			})

			Context("when WriteOnFailure is set", func() {
				BeforeEach(func() {
					qJob.Spec.Output = &qjv1a1.Output{
						WriteOnFailure: true,
						OutputMap:      env.DefaultOutputMap(),
					}
				})

				It("does persist the output", func() {
					err := po.Persist(context.Background())
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "foo-busybox", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "fake-nats", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					_, err = clientSet.CoreV1().Secrets(namespace).Get(context.Background(), "bar-nuts-v1", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			AfterEach(func() {
				Expect(os.RemoveAll(tmpDir)).ToNot(HaveOccurred())
			})
		})
	})
})
