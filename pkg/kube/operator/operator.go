package operator

import (
	"context"

	"github.com/pkg/errors"

	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/crd"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(ctx context.Context, config *config.Config, cfg *rest.Config, options manager.Options) (manager.Manager, error) {
	mgr, err := manager.New(cfg, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize new manager")
	}

	log := ctxlog.ExtractLogger(ctx)

	log.Info("Registering Components")
	config.Namespace = options.Namespace

	// Setup Scheme for all resources
	err = controllers.AddToScheme(mgr.GetScheme())
	if err != nil {
		return nil, errors.Wrap(err, "failed to add manager scheme to controllers")
	}

	// Setup all Controllers
	err = controllers.AddToManager(ctx, config, mgr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add controllers to manager")
	}

	return mgr, nil
}

// ApplyCRDs applies a collection of CRDs into the cluster
func ApplyCRDs(config *rest.Config) error {
	exClient, err := extv1client.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	err = crd.ApplyCRD(
		exClient,
		ejv1.ExtendedJobResourceName,
		ejv1.ExtendedJobResourceKind,
		ejv1.ExtendedJobResourcePlural,
		ejv1.ExtendedJobResourceShortNames,
		ejv1.SchemeGroupVersion,
		&ejv1.ExtendedJobValidation,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", ejv1.ExtendedJobResourceName)
	}
	err = crd.WaitForCRDReady(exClient, ejv1.ExtendedJobResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", ejv1.ExtendedJobResourceName)
	}

	return nil
}
