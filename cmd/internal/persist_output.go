package cmd

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/quarksjob"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"code.cloudfoundry.org/quarks-utils/pkg/kubeconfig"
)

// persistOutputCmd is the persist-output command.
var persistOutputCmd = &cobra.Command{
	Use:   "persist-output [flags]",
	Short: "Persist a file into a kube secret",
	Long: `Persists a log file created by containers in a pod of quarksJob
	
into a versioned secret or kube native secret using flags specified to this command.
`,
	RunE: func(_ *cobra.Command, args []string) (err error) {

		namespace := viper.GetString("operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("persist-output command failed. operator-namespace flag is empty.")
		}

		// hostname of the container is the pod name in kubernetes
		podName, err := os.Hostname()
		if err != nil {
			return errors.Wrapf(err, "failed to fetch pod name.")
		}
		if podName == "" {
			return errors.Wrapf(err, "pod name is empty.")
		}

		log := cmd.Logger(zap.AddCallerSkip(1))
		defer log.Sync()

		// Authenticate with the cluster
		clientSet, versionedClientSet, err := authenticateInCluster(log)
		if err != nil {
			return err
		}

		po := quarksjob.NewOutputPersistor(log, namespace, podName, clientSet, versionedClientSet, "/mnt/quarks")

		return po.Persist()
	},
}

func init() {
	rootCmd.AddCommand(persistOutputCmd)
}

// authenticateInCluster authenticates with the in cluster and returns the client
func authenticateInCluster(log *zap.SugaredLogger) (*kubernetes.Clientset, *versioned.Clientset, error) {
	config, err := kubeconfig.NewGetter(log).Get("")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Couldn't fetch Kubeconfig. Ensure kubeconfig is present to continue.")
	}
	if err := kubeconfig.NewChecker(log).Check(config); err != nil {
		return nil, nil, errors.Wrapf(err, "Couldn't check Kubeconfig. Ensure kubeconfig is correct to continue.")
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create clientset with incluster config")
	}

	versionedClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create versioned clientset with incluster config")
	}

	return clientSet, versionedClientSet, nil
}
