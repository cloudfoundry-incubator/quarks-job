package environment

import (
	"sync/atomic"
	"time"

	gomegaConfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-job/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-job/pkg/kube/util/config"
	"code.cloudfoundry.org/quarks-job/testing"
	sharedcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// Environment test env with helpers to create structs and k8s resources
type Environment struct {
	*utils.Environment
	Machine
	testing.Catalog
	Config *config.Config
}

var (
	namespaceCounter int32
)

// NewEnvironment returns a new test environment
func NewEnvironment(kubeConfig *rest.Config) *Environment {
	atomic.AddInt32(&namespaceCounter, 1)
	namespaceID := gomegaConfig.GinkgoConfig.ParallelNode*100 + int(namespaceCounter)
	shared := &sharedcfg.Config{
		CtxTimeOut:           10 * time.Second,
		MeltdownDuration:     1 * time.Second,
		MeltdownRequeueAfter: 500 * time.Millisecond,
		Fs:                   afero.NewOsFs(),
	}

	return &Environment{
		Environment: &utils.Environment{
			ID:         namespaceID,
			Namespace:  utils.GetNamespaceName(namespaceID),
			KubeConfig: kubeConfig,
			Config:     shared,
		},
		Machine: Machine{
			Machine: machine.NewMachine(),
		},
		Config: &config.Config{
			Config: shared,
		},
	}
}

// SetupClientsets initializes kube clientsets
func (e *Environment) SetupClientsets() error {
	var err error
	e.Clientset, err = kubernetes.NewForConfig(e.KubeConfig)
	if err != nil {
		return err
	}

	e.VersionedClientset, err = versioned.NewForConfig(e.KubeConfig)
	if err != nil {
		return err
	}

	return nil
}

// SetupNamespace creates the namespace and the clientsets and prepares the teardowm
func (e *Environment) SetupNamespace() error {
	nsTeardown, err := e.CreateNamespace(e.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Creating namespace %s failed", e.Namespace)
	}

	e.Teardown = func(wasFailure bool) {
		if wasFailure {
			utils.DumpENV(e.Namespace)
		}

		err := nsTeardown()
		if err != nil {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		if e.Stop != nil {
			close(e.Stop)
		}
	}

	return nil
}

// ApplyCRDs applies the CRDs to the cluster
func ApplyCRDs(kubeConfig *rest.Config) error {
	err := operator.ApplyCRDs(kubeConfig)
	if err != nil {
		return err
	}
	return nil
}
