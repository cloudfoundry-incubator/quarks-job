package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
)

// Theses funcs construct controllers and add them to the controller-runtime
// manager. The manager will set fields on the controllers and start them, when
// itself is started.
var addToManagerFuncs = []func(context.Context, *config.Config, manager.Manager) error{
	extendedjob.AddErrand,
	extendedjob.AddJob,
	extendedjob.AddTrigger,
}

var addToSchemes = runtime.SchemeBuilder{
	ejv1.AddToScheme,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(ctx context.Context, config *config.Config, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(ctx, config, m); err != nil {
			return err
		}
	}
	return nil
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}
