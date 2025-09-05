package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/amartyaa/mck/pkg/output"
	"github.com/amartyaa/mck/pkg/provider"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	listProvider string
	listRegion   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Kubernetes clusters across all configured providers",
	Long: `List all Kubernetes clusters from AWS EKS, Azure AKS, GCP GKE, and Oracle OKE.

Examples:
  mck list                          # List all clusters from all providers
  mck list --provider aws           # List only AWS EKS clusters
  mck list --provider azure -o json # List Azure clusters as JSON
  mck list --region us-east-1       # Filter by region`,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		registry := initProviders()
		providers := registry.All()

		// Filter by provider if specified
		if listProvider != "" {
			p, err := registry.Get(listProvider)
			if err != nil {
				return fmt.Errorf("unknown provider: %s (available: %v)", listProvider, registry.Names())
			}
			providers = []provider.Provider{p}
		}

		var allClusters []provider.ClusterInfo
		for _, p := range providers {
			clusters, err := p.ListClusters(ctx)
			if err != nil {
				color.Yellow("⚠ %s: %v", p.Name(), err)
				continue
			}
			allClusters = append(allClusters, clusters...)
		}

		// Filter by region if specified
		if listRegion != "" {
			var filtered []provider.ClusterInfo
			for _, c := range allClusters {
				if c.Region == listRegion {
					filtered = append(filtered, c)
				}
			}
			allClusters = filtered
		}

		if len(allClusters) == 0 {
			fmt.Println("No clusters found.")
			return nil
		}

		// Render output
		format := output.ParseFormat(outputFormat)
		columns := []output.Column{
			{Header: "NAME", Width: 25},
			{Header: "PROVIDER", Width: 8},
			{Header: "REGION", Width: 15},
			{Header: "VERSION", Width: 8},
			{Header: "STATUS", Width: 12},
			{Header: "NODES", Width: 5},
		}

		rows := make([][]string, len(allClusters))
		for i, c := range allClusters {
			rows[i] = []string{
				c.Name,
				c.Provider,
				c.Region,
				c.Version,
				output.StatusColor(c.Status),
				fmt.Sprintf("%d", c.NodeCount),
			}
		}

		return output.Render(os.Stdout, format, columns, rows, allClusters)
	},
}

func init() {
	listCmd.Flags().StringVarP(&listProvider, "provider", "p", "", "filter by provider: aws, gcp, azure, oci")
	listCmd.Flags().StringVarP(&listRegion, "region", "r", "", "filter by region")
	rootCmd.AddCommand(listCmd)
}
