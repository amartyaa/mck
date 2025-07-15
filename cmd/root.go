package cmd

import (
"fmt"
"os"

"github.com/spf13/cobra"
"github.com/spf13/viper"
)

var (
cfgFile string
output  string
)

var rootCmd = &cobra.Command{
Use:   "mck",
Short: "Multi-Cloud Kubernetes CLI",
Long: `mck is a unified CLI for managing Kubernetes clusters across
multiple cloud providers - AWS EKS, Azure AKS, GCP GKE, and Oracle OKE.

One tool. Every cloud. All your clusters.`,
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
rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
Use:   "version",
Short: "Print the version of mck",
Run: func(cmd *cobra.Command, args []string) {
fmt.Println("mck v0.1.0")
},
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
