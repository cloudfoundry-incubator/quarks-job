package cli_test

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CLI", func() {
	act := func(arg ...string) (session *gexec.Session, err error) {
		cmd := exec.Command(cliPath, arg...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	BeforeEach(func() {
		os.Setenv("DOCKER_IMAGE_TAG", "v0.0.0")
	})

	Describe("help", func() {
		It("should show the help for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Usage:`))
		})

		It("should show all available options for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
      --apply-crd                         \(APPLY_CRD\) If true, apply CRDs on start \(default true\)
      --ctx-timeout int                   \(CTX_TIMEOUT\) context timeout for each k8s API request in seconds \(default 30\)
  -o, --docker-image-org string           \(DOCKER_IMAGE_ORG\) Dockerhub organization that provides the operator docker image \(default "cfcontainerization"\)
      --docker-image-pull-policy string   \(DOCKER_IMAGE_PULL_POLICY\) Image pull policy \(default "IfNotPresent"\)
  -r, --docker-image-repository string    \(DOCKER_IMAGE_REPOSITORY\) Dockerhub repository that provides the operator docker image \(default "cf-operator"\)
  -t, --docker-image-tag string           \(DOCKER_IMAGE_TAG\) Tag of the operator docker image
  -h, --help                              help for quarks-job
  -c, --kubeconfig string                 \(KUBECONFIG\) Path to a kubeconfig, not required in-cluster
  -l, --log-level string                  \(LOG_LEVEL\) Only print log messages from this level onward \(default "debug"\)
      --max-workers int                   \(MAX_WORKERS\) Maximum number of workers concurrently running the controller \(default 1\)
  -n, --operator-namespace string         \(OPERATOR_NAMESPACE\) The operator namespace \(default "default"\)
      --watch-namespace string            \(WATCH_NAMESPACE\) Namespace to watch for BOSH deployments

`))
		})

		It("shows all available commands", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Available Commands:
  help           Help about any command
  persist-output Persist a file into a kube secret
  version        Print the version number

`))
		})
	})

	Describe("default", func() {

		It("should start the server", func() {
			session, err := act()
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say(`Starting quarks-job \d+\.\d+\.\d+ with namespace`))
			Eventually(session.Err).ShouldNot(Say(`Applying CRDs...`))
		})

		Context("when specifying namespace", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("OPERATOR_NAMESPACE", "env-test")
				})

				AfterEach(func() {
					os.Setenv("OPERATOR_NAMESPACE", "")
				})

				It("should start for namespace", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting quarks-job \d+\.\d+\.\d+ with namespace env-test`))
				})
			})

			Context("via using switches", func() {
				It("should start for namespace", func() {
					session, err := act("--operator-namespace", "switch-test")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting quarks-job \d+\.\d+\.\d+ with namespace switch-test`))
				})
			})
		})

		Context("when enabling apply-crd", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("APPLY_CRD", "true")
				})

				AfterEach(func() {
					os.Setenv("APPLY_CRD", "")
				})

				It("should apply CRDs", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Applying CRDs...`))
				})
			})

			Context("via using switches", func() {
				It("should apply CRDs", func() {
					session, err := act("--apply-crd")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Applying CRDs...`))
				})
			})
		})
	})

	Describe("version", func() {
		It("should show a semantic version number", func() {
			session, err := act("version")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Quarks-Job Version: \d+.\d+.\d+`))
		})
	})
})
