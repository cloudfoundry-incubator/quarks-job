package operator

import (
	"context"

	"github.com/pkg/errors"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/crd"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
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
func ApplyCRDs(ctx context.Context, config *rest.Config) error {
	client, err := extv1client.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	b := crd.New(
		qjv1a1.QuarksJobResourceName,
		extv1.CustomResourceDefinitionNames{
			Kind:       qjv1a1.QuarksJobResourceKind,
			Plural:     qjv1a1.QuarksJobResourcePlural,
			ShortNames: qjv1a1.QuarksJobResourceShortNames,
		},
		qjv1a1.SchemeGroupVersion,
	)

	err = b.WithValidation(&qjv1a1.QuarksJobValidation).
		WithAdditionalPrinterColumns(qjv1a1.QuarksJobAdditionalPrinterColumns).
		Build().
		Apply(ctx, client)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", qjv1a1.QuarksJobResourceName)
	}

	err = crd.WaitForCRDReady(ctx, client, qjv1a1.QuarksJobResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", qjv1a1.QuarksJobResourceName)
	}

	return nil
}
