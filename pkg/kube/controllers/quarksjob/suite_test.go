package quarksjob_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestQuarksJob(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuarksJob Suite")
}

var _ = BeforeSuite(func() {
	// setup logging for controller-runtime
	logf.SetLogger(zap.Logger(true))
})
