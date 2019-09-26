package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"code.cloudfoundry.org/quarks-job/version"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Quarks-Job Version: %s\n", version.Version)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}
