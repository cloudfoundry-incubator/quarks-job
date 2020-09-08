package cmd

import (
	golog "log"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"code.cloudfoundry.org/quarks-job/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-job/version"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
)

var log *zap.SugaredLogger

func wrapError(err error, msg string) error {
	return errors.Wrapf(err, "quarks-job command failed. %s", msg)
}

var rootCmd = &cobra.Command{
	Use:   "quarks-job",
	Short: "quarks-job starts the operator",
	RunE: func(_ *cobra.Command, args []string) error {
		log = logger.NewControllerLogger(cmd.LogLevel())
		defer func() {
			_ = log.Sync()
		}()

		restConfig, err := cmd.KubeConfig(log)
		if err != nil {
			return wrapError(err, "")
		}

		cfg := config.NewDefaultConfig(afero.NewOsFs())

		cmd.MonitoredID(cfg)

		log.Infof("Starting quarks-job %s, monitoring namespaces labeled with '%s'", version.Version, cfg.MonitoredID)

		err = cmd.DockerImage()
		if err != nil {
			return wrapError(err, "")
		}
		log.Infof("quarks-job docker image: %s", config.GetOperatorDockerImage())

		cfg.MaxQuarksJobWorkers = viper.GetInt("max-workers")

		cmd.CtxTimeOut(cfg)
		cmd.Meltdown(cfg)

		ctx := ctxlog.NewParentContext(log)

		err = cmd.ApplyCRDs(ctx, operator.ApplyCRDs, restConfig)
		if err != nil {
			return wrapError(err, "Couldn't apply CRDs.")
		}

		mgr, err := operator.NewManager(ctx, cfg, restConfig, manager.Options{
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

// NewOperatorCommand returns the `quarks-job` command.
func NewOperatorCommand() *cobra.Command {
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

	argToEnv := map[string]string{}

	cmd.CtxTimeOutFlags(pf, argToEnv)
	cmd.MonitoredIDFlags(pf, argToEnv)
	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.LoggerFlags(pf, argToEnv)
	cmd.DockerImageFlags(pf, argToEnv, "quarks-job", version.Version)
	cmd.ApplyCRDsFlags(pf, argToEnv)
	cmd.MeltdownFlags(pf, argToEnv)

	pf.Int("max-workers", 1, "Maximum number of workers concurrently running the controller")
	viper.BindPFlag("max-workers", pf.Lookup("max-workers"))
	argToEnv["max-workers"] = "MAX_WORKERS"

	// Add env variables to help
	cmd.AddEnvToUsage(rootCmd, argToEnv)

	// Do not display cmd usage and errors
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}
