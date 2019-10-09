package cmd

import (
	"fmt"
	golog "log"
	"os"

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/quarks-job/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-job/version"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	kubeConfig "code.cloudfoundry.org/quarks-utils/pkg/kubeconfig"
)

var log *zap.SugaredLogger

func wrapError(err error, msg string) error {
	return errors.Wrap(err, "quarks-job command failed. "+msg)
}

var rootCmd = &cobra.Command{
	Use:   "quarks-job",
	Short: "quarks-job starts the operator",
	RunE: func(cmd *cobra.Command, args []string) error {
		log = newLogger(zap.AddCallerSkip(1))
		defer log.Sync()

		restConfig, err := kubeConfig.NewGetter(log).Get(viper.GetString("kubeconfig"))
		if err != nil {
			return wrapError(err, "Couldn't fetch Kubeconfig. Ensure kubeconfig is present to continue.")
		}
		if err := kubeConfig.NewChecker(log).Check(restConfig); err != nil {
			return wrapError(err, "Couldn't check Kubeconfig. Ensure kubeconfig is correct to continue.")
		}

		operatorNamespace := viper.GetString("operator-namespace")
		watchNamespace := viper.GetString("watch-namespace")
		if watchNamespace == "" {
			log.Infof("No watch namespace defined. Falling back to the operator namespace.")
			watchNamespace = operatorNamespace
		}

		dockerImageTag := viper.GetString("docker-image-tag")
		if dockerImageTag == "" {
			return errors.Errorf("environment variable DOCKER_IMAGE_TAG not set")
		}

		err = extendedjob.SetupOperatorDockerImage(
			viper.GetString("docker-image-org"),
			viper.GetString("docker-image-repository"),
			dockerImageTag,
		)
		if err != nil {
			return wrapError(err, "Couldn't parse quarks-job docker image reference.")
		}

		log.Infof("Starting quarks-job %s with namespace %s", version.Version, watchNamespace)
		log.Infof("quarks-job docker image: %s", extendedjob.GetOperatorDockerImage())

		cfg := config.NewJobConfig(
			watchNamespace,
			operatorNamespace,
			viper.GetInt("ctx-timeout"),
			afero.NewOsFs(),
			viper.GetInt("max-workers"),
		)
		ctx := ctxlog.NewParentContext(log)

		if viper.GetBool("apply-crd") {
			ctxlog.Info(ctx, "Applying CRDs...")
			err := operator.ApplyCRDs(restConfig)
			if err != nil {
				return wrapError(err, "Couldn't apply CRDs.")
			}
		}

		mgr, err := operator.NewManager(ctx, cfg, restConfig, manager.Options{
			Namespace:          watchNamespace,
			MetricsBindAddress: "0",
			LeaderElection:     false,
		})
		if err != nil {
			return wrapError(err, "Failed to create new manager.")
		}

		ctxlog.Info(ctx, "Waiting for Quarks job resources...")

		err = mgr.Start(signals.SetupSignalHandler())
		if err != nil {
			return wrapError(err, "Failed to start quarks-job manager.")
		}
		return nil
	},
	TraverseChildren: true,
}

// NewCFOperatorCommand returns the `quarks-job` command.
func NewCFOperatorCommand() *cobra.Command {
	return rootCmd
}

// Execute the root command, runs the server
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		golog.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()

	pf.Bool("apply-crd", true, "If true, apply CRDs on start")
	pf.Int("ctx-timeout", 30, "context timeout for each k8s API request in seconds")
	pf.StringP("operator-namespace", "n", "default", "The operator namespace")
	pf.StringP("docker-image-org", "o", "cfcontainerization", "Dockerhub organization that provides the operator docker image")
	pf.StringP("docker-image-repository", "r", "cf-operator", "Dockerhub repository that provides the operator docker image")
	pf.StringP("docker-image-tag", "t", "", "Tag of the operator docker image")
	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("log-level", "l", "debug", "Only print log messages from this level onward")
	pf.Int("max-workers", 1, "Maximum number of workers concurrently running the controller")
	pf.StringP("watch-namespace", "", "", "Namespace to watch for BOSH deployments")

	viper.BindPFlag("apply-crd", rootCmd.PersistentFlags().Lookup("apply-crd"))
	viper.BindPFlag("ctx-timeout", pf.Lookup("ctx-timeout"))
	viper.BindPFlag("operator-namespace", pf.Lookup("operator-namespace"))
	viper.BindPFlag("docker-image-org", pf.Lookup("docker-image-org"))
	viper.BindPFlag("docker-image-repository", pf.Lookup("docker-image-repository"))
	viper.BindPFlag("docker-image-tag", rootCmd.PersistentFlags().Lookup("docker-image-tag"))
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("log-level", pf.Lookup("log-level"))
	viper.BindPFlag("max-workers", pf.Lookup("max-workers"))
	viper.BindPFlag("watch-namespace", pf.Lookup("watch-namespace"))

	argToEnv := map[string]string{
		"apply-crd":               "APPLY_CRD",
		"ctx-timeout":             "CTX_TIMEOUT",
		"operator-namespace":      "OPERATOR_NAMESPACE",
		"docker-image-org":        "DOCKER_IMAGE_ORG",
		"docker-image-repository": "DOCKER_IMAGE_REPOSITORY",
		"docker-image-tag":        "DOCKER_IMAGE_TAG",
		"kubeconfig":              "KUBECONFIG",
		"log-level":               "LOG_LEVEL",
		"max-workers":             "MAX_WORKERS",
		"watch-namespace":         "WATCH_NAMESPACE",
	}

	// Add env variables to help
	AddEnvToUsage(rootCmd, argToEnv)

	// Do not display cmd usage and errors
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}

// newLogger returns a new zap logger
func newLogger(options ...zap.Option) *zap.SugaredLogger {
	level := viper.GetString("log-level")
	l := zap.DebugLevel
	l.Set(level)

	cfg := zap.NewDevelopmentConfig()
	cfg.Development = false
	cfg.Level = zap.NewAtomicLevelAt(l)
	logger, err := cfg.Build(options...)
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}

	// Make controller-runtime log using our logger
	crlog.SetLogger(zapr.NewLogger(logger))

	return logger.Sugar()
}

// AddEnvToUsage adds env variables to help
func AddEnvToUsage(cfOperatorCommand *cobra.Command, argToEnv map[string]string) {
	flagSet := make(map[string]bool)

	for arg, env := range argToEnv {
		viper.BindEnv(arg, env)
		flag := cfOperatorCommand.Flag(arg)

		if flag != nil {
			flagSet[flag.Name] = true
			// add environment variable to the description
			flag.Usage = fmt.Sprintf("(%s) %s", env, flag.Usage)
		}
	}
}
