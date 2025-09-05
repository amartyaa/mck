package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "mck",
	Short: "Multi-Cloud Kubernetes CLI",
	Long: `mck is a unified CLI for managing Kubernetes clusters across
multiple cloud providers — AWS EKS, Azure AKS, GCP GKE, and Oracle OKE.

One tool. Every cloud. All your clusters.

Commands:
  list       List clusters across all providers
  connect    Connect to a cluster and set kubectl context
  status     Show node and resource status
  apply      Deploy manifests to multiple clusters
  cost       View cost data across providers
  version    Print mck version`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.mck.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format: table, json, yaml")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".mck")
	}
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}
