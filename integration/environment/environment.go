package environment

import (
	"time"

	utils "code.cloudfoundry.org/cf-operator/integration/environment"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-job/testing"
)

type Environment struct {
	*utils.Environment
	Machine
	testing.Catalog
}

func NewEnvironment(kubeConfig *rest.Config) *Environment {
	return &Environment{
		Environment: utils.NewEnvironment(kubeConfig),
		Machine: Machine{
			pollTimeout:  300 * time.Second,
			pollInterval: 500 * time.Millisecond,
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
