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

	kubeConfig "code.cloudfoundry.org/cf-operator/pkg/kube/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"

	"code.cloudfoundry.org/quarks-job/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-job/version"
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

		cfOperatorNamespace := viper.GetString("namespace")

		log.Infof("Starting quarks-job %s with namespace %s", version.Version, cfOperatorNamespace)

		cfg := &config.Config{
			Namespace:             cfOperatorNamespace,
			Fs:                    afero.NewOsFs(),
			MaxExtendedJobWorkers: viper.GetInt("max-workers"),
			ApplyCRD:              viper.GetBool("apply-crd"),
			CtxTimeOut:            config.CtxTimeOut,
			MeltdownDuration:      config.MeltdownDuration,
			MeltdownRequeueAfter:  config.MeltdownRequeueAfter,
		}
		ctx := ctxlog.NewParentContext(log)

		if viper.GetBool("apply-crd") {
			ctxlog.Info(ctx, "Applying CRDs...")
			err := operator.ApplyCRDs(restConfig)
			if err != nil {
				return wrapError(err, "Couldn't apply CRDs.")
			}
		}

		mgr, err := operator.NewManager(ctx, cfg, restConfig, manager.Options{
			Namespace:          cfOperatorNamespace,
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

	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("log-level", "l", "debug", "Only print log messages from this level onward")
	pf.StringP("namespace", "n", "default", "Namespace to watch")
	pf.Int("max-workers", 1, "Maximum number of workers concurrently running the controller")
	pf.Bool("apply-crd", true, "If true, apply CRDs on start")
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("log-level", pf.Lookup("log-level"))
	viper.BindPFlag("namespace", pf.Lookup("namespace"))
	viper.BindPFlag("max-workers", pf.Lookup("max-workers"))
	viper.BindPFlag("apply-crd", rootCmd.PersistentFlags().Lookup("apply-crd"))

	argToEnv := map[string]string{
		"kubeconfig":  "KUBECONFIG",
		"log-level":   "LOG_LEVEL",
		"namespace":   "NAMESPACE",
		"max-workers": "MAX_WORKERS",
		"apply-crd":   "APPLY_CRD",
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
