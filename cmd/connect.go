package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/amartyaa/mck/pkg/kubeconfig"
	"github.com/amartyaa/mck/pkg/provider"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	connectFile   string
	connectMerge  bool
)

var connectCmd = &cobra.Command{
	Use:   "connect [cluster-name]",
	Short: "Connect to a Kubernetes cluster by name",
	Long: `Fetch kubeconfig for the named cluster and set it as the active context.

The cluster is searched across all configured providers. Use aliases for quick access.

Examples:
  mck connect prod-eks-us-east          # Connect and set kubectl context
  mck connect staging-aks --merge       # Merge into existing kubeconfig
  mck connect dev-gke -f ./kubeconfig   # Save to specific file`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := args[0]

		// Check if it's an alias
		clusterName = resolveAlias(clusterName)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		registry := initProviders()

		// Search all providers for the cluster
		var kubecfg []byte
		var foundProvider string

		for _, p := range registry.All() {
			kc, err := p.GetKubeconfig(ctx, clusterName)
			if err == nil && len(kc) > 0 {
				kubecfg = kc
				foundProvider = p.Name()
				break
			}
		}

		if kubecfg == nil {
			return fmt.Errorf("cluster %q not found in any configured provider", clusterName)
		}

		// Save or merge kubeconfig
		if connectFile != "" {
			if err := kubeconfig.SaveToFile(kubecfg, connectFile); err != nil {
				return fmt.Errorf("saving kubeconfig: %w", err)
			}
			color.Green("✓ Kubeconfig saved to %s", connectFile)
		} else if connectMerge {
			if err := kubeconfig.MergeToDefault(kubecfg, clusterName); err != nil {
				return fmt.Errorf("merging kubeconfig: %w", err)
			}
			color.Green("✓ Merged kubeconfig for %s (%s)", clusterName, foundProvider)
		} else {
			// Default: merge and set context
			if err := kubeconfig.MergeToDefault(kubecfg, clusterName); err != nil {
				return fmt.Errorf("merging kubeconfig: %w", err)
			}
			if err := kubeconfig.SetContext(clusterName); err != nil {
				color.Yellow("⚠ Kubeconfig merged but could not switch context: %v", err)
			} else {
				color.Green("✓ Connected to %s (%s) — kubectl is ready", clusterName, foundProvider)
			}
		}

		return nil
	},
}

func resolveAlias(name string) string {
	// Check viper config for aliases
	aliases := viperGetStringMap("aliases")
	if target, ok := aliases[name]; ok {
		color.Cyan("→ Resolved alias %q to %q", name, target)
		// Parse "provider:region:cluster" format
		parts := splitAlias(target)
		if len(parts) == 3 {
			return parts[2] // Return just the cluster name
		}
		return target
	}
	return name
}

func splitAlias(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func viperGetStringMap(key string) map[string]string {
	// Simplified — in production this reads from viper
	return make(map[string]string)
}

func init() {
	connectCmd.Flags().StringVarP(&connectFile, "file", "f", "", "save kubeconfig to specific file instead of merging")
	connectCmd.Flags().BoolVar(&connectMerge, "merge", false, "merge into existing kubeconfig without switching context")
	rootCmd.AddCommand(connectCmd)
}

// initProviders initializes all configured providers from config.
// In a full implementation, this reads from viper/config file.
func initProviders() *provider.Registry {
	registry := provider.NewRegistry()

	// Initialize providers based on config
	// AWS
	awsRegions := viperGetStringSlice("providers.aws.regions")
	if len(awsRegions) > 0 {
		awsProfile := viperGetString("providers.aws.profile")
		registry.Register(provider.NewAWSProvider(awsProfile, awsRegions))
	}

	// GCP
	gcpProjects := viperGetStringSlice("providers.gcp.projects")
	if len(gcpProjects) > 0 {
		gcpZones := viperGetStringSlice("providers.gcp.zones")
		registry.Register(provider.NewGCPProvider(gcpProjects, gcpZones))
	}

	// Azure
	azureSubs := viperGetStringSlice("providers.azure.subscription_ids")
	if len(azureSubs) > 0 {
		azureRGs := viperGetStringSlice("providers.azure.resource_groups")
		registry.Register(provider.NewAzureProvider(azureSubs, azureRGs))
	}

	// OCI
	ociCompartments := viperGetStringSlice("providers.oci.compartments")
	if len(ociCompartments) > 0 {
		ociProfile := viperGetString("providers.oci.profile")
		registry.Register(provider.NewOCIProvider(ociCompartments, ociProfile))
	}

	return registry
}

// Viper helpers — read from the global viper config
func viperGetStringSlice(key string) []string {
	return viper.GetStringSlice(key)
}

func viperGetString(key string) string {
	return viper.GetString(key)
}

var outputFormat string

func init() {
	// This is set via persistent flag in root.go
}
