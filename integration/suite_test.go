package integration_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-job/integration/environment"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	env              *environment.Environment
	namespacesToNuke []string
)

var _ = BeforeEach(func() {
	env = environment.SetupNamespace()
	namespacesToNuke = append(namespacesToNuke, env.Namespace)
})

var _ = AfterEach(func() {
	env.Teardown(CurrentGinkgoTestDescription().Failed)
})

var _ = AfterSuite(func() {
	// Nuking all namespaces at the end of the run
	for _, namespace := range namespacesToNuke {
		err := cmdHelper.DeleteNamespace(namespace)
		if err != nil {
			fmt.Printf("WARNING: failed to delete namespace %s: %v\n", namespace, err)
		}
	}
})
