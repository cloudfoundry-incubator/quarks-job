package environment

import (
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/quarks-job/pkg/kube/operator"
)

// StartOperator starts the extended job operator
func (e *Environment) StartOperator() (chan struct{}, error) {
	mgr, err := e.setupCFOperator()
	if err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	go func() {
		err := mgr.Start(stop)
		if err != nil {
			panic(err)
		}
	}()
	return stop, err
}

func (e *Environment) setupCFOperator() (manager.Manager, error) {
	ctx := e.SetupLoggerContext("quarks-job")

	mgr, err := operator.NewManager(ctx, e.Config, e.KubeConfig, manager.Options{
		Namespace:          e.Namespace,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Host:               "0.0.0.0",
	})

	return mgr, err
}
