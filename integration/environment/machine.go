package environment

import (
	"time"

	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
)

// Machine produces and destroys resources for tests
type Machine struct {
	pollTimeout  time.Duration
	pollInterval time.Duration

	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}
