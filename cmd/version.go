package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// These are set at build time via ldflags.
var (
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of mck",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mck %s\n", Version)
		fmt.Printf("  commit:  %s\n", GitCommit)
		fmt.Printf("  built:   %s\n", BuildDate)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
