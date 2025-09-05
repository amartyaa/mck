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

var statusAll bool

var statusCmd = &cobra.Command{
	Use:   "status [cluster-name]",
	Short: "Show node and resource status for a cluster",
	Long: `Display node-level status information for a specific cluster, or an overview of all clusters.

Examples:
  mck status prod-eks                # Show nodes for a specific cluster
  mck status --all                   # Show summary status of all clusters
  mck status prod-eks -o json        # Node status as JSON`,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		registry := initProviders()

		if statusAll || len(args) == 0 {
			return showAllStatus(ctx, registry)
		}

		clusterName := resolveAlias(args[0])
		return showClusterStatus(ctx, registry, clusterName)
	},
}

func showAllStatus(ctx context.Context, registry *provider.Registry) error {
	var allClusters []provider.ClusterInfo

	for _, p := range registry.All() {
		clusters, err := p.ListClusters(ctx)
		if err != nil {
			color.Yellow("⚠ %s: %v", p.Name(), err)
			continue
		}
		allClusters = append(allClusters, clusters...)
	}

	if len(allClusters) == 0 {
		fmt.Println("No clusters found.")
		return nil
	}

	format := output.ParseFormat(outputFormat)
	columns := []output.Column{
		{Header: "CLUSTER", Width: 25},
		{Header: "PROVIDER", Width: 8},
		{Header: "REGION", Width: 15},
		{Header: "VERSION", Width: 8},
		{Header: "STATUS", Width: 12},
		{Header: "NODES", Width: 5},
		{Header: "ENDPOINT", Width: 30},
	}

	rows := make([][]string, len(allClusters))
	for i, c := range allClusters {
		endpoint := c.Endpoint
		if len(endpoint) > 30 {
			endpoint = endpoint[:27] + "..."
		}
		rows[i] = []string{
			c.Name,
			c.Provider,
			c.Region,
			c.Version,
			output.StatusColor(c.Status),
			fmt.Sprintf("%d", c.NodeCount),
			endpoint,
		}
	}

	return output.Render(os.Stdout, format, columns, rows, allClusters)
}

func showClusterStatus(ctx context.Context, registry *provider.Registry, clusterName string) error {
	for _, p := range registry.All() {
		nodes, err := p.GetNodes(ctx, clusterName)
		if err != nil {
			continue
		}

		// Found the cluster
		cluster, _ := p.GetCluster(ctx, clusterName)
		if cluster != nil {
			color.Cyan("Cluster: %s (%s / %s)", cluster.Name, cluster.Provider, cluster.Region)
			color.Cyan("Version: %s   Status: %s   Nodes: %d",
				cluster.Version, output.StatusColor(cluster.Status), len(nodes))
			fmt.Println()
		}

		format := output.ParseFormat(outputFormat)
		columns := []output.Column{
			{Header: "NODE", Width: 30},
			{Header: "INSTANCE TYPE", Width: 20},
			{Header: "ZONE", Width: 15},
			{Header: "STATUS", Width: 12},
			{Header: "CPU", Width: 8},
			{Header: "MEMORY", Width: 8},
			{Header: "PODS", Width: 5},
		}

		rows := make([][]string, len(nodes))
		for i, n := range nodes {
			rows[i] = []string{
				n.Name,
				n.InstanceType,
				n.Zone,
				output.StatusColor(n.Status),
				n.CPUUsed + "/" + n.CPUCapacity,
				n.MemUsed + "/" + n.MemCapacity,
				fmt.Sprintf("%d", n.Pods),
			}
		}

		return output.Render(os.Stdout, format, columns, rows, nodes)
	}

	return fmt.Errorf("cluster %q not found in any configured provider", clusterName)
}

func init() {
	statusCmd.Flags().BoolVar(&statusAll, "all", false, "show status of all clusters")
	rootCmd.AddCommand(statusCmd)
}
