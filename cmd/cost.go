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
	costMonth    string
	costProvider string
)

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Show cost data for Kubernetes clusters",
	Long: `Display cost information for clusters across cloud providers.
Requires cost APIs to be configured for each provider.

Examples:
  mck cost                              # Current month, all providers
  mck cost --month 2025-08              # Specific month
  mck cost --provider aws -o json       # AWS costs as JSON`,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		registry := initProviders()

		// Default to current month
		if costMonth == "" {
			costMonth = time.Now().Format("2006-01")
		}

		providers := registry.All()
		if costProvider != "" {
			p, err := registry.Get(costProvider)
			if err != nil {
				return fmt.Errorf("unknown provider: %s", costProvider)
			}
			providers = []provider.Provider{p}
		}

		type costRow struct {
			Cluster  string  `json:"cluster" yaml:"cluster"`
			Provider string  `json:"provider" yaml:"provider"`
			Region   string  `json:"region" yaml:"region"`
			Cost     float64 `json:"monthly_cost" yaml:"monthly_cost"`
			Currency string  `json:"currency" yaml:"currency"`
		}

		var rows []costRow
		totalCost := 0.0

		for _, p := range providers {
			clusters, err := p.ListClusters(ctx)
			if err != nil {
				color.Yellow("⚠ %s: %v", p.Name(), err)
				continue
			}

			for _, cluster := range clusters {
				cost, err := p.GetCost(ctx, cluster.Name, costMonth)
				if err != nil {
					color.Yellow("⚠ %s/%s: %v", p.Name(), cluster.Name, err)
					continue
				}

				rows = append(rows, costRow{
					Cluster:  cluster.Name,
					Provider: p.Name(),
					Region:   cluster.Region,
					Cost:     cost.MonthlyEstimate,
					Currency: cost.Currency,
				})
				totalCost += cost.MonthlyEstimate
			}
		}

		if len(rows) == 0 {
			fmt.Println("No cost data available. Ensure provider cost APIs are configured.")
			return nil
		}

		format := output.ParseFormat(outputFormat)
		columns := []output.Column{
			{Header: "CLUSTER", Width: 25},
			{Header: "PROVIDER", Width: 8},
			{Header: "REGION", Width: 15},
			{Header: "MONTHLY COST", Width: 14},
			{Header: "CURRENCY", Width: 8},
		}

		tableRows := make([][]string, len(rows))
		for i, r := range rows {
			tableRows[i] = []string{
				r.Cluster,
				r.Provider,
				r.Region,
				fmt.Sprintf("%.2f", r.Cost),
				r.Currency,
			}
		}

		// Add total row
		tableRows = append(tableRows, []string{
			"", "", color.New(color.Bold).Sprint("TOTAL"),
			color.New(color.Bold).Sprintf("%.2f", totalCost),
			"USD",
		})

		return output.Render(os.Stdout, format, columns, tableRows, rows)
	},
}

func init() {
	costCmd.Flags().StringVar(&costMonth, "month", "", "month in YYYY-MM format (default: current)")
	costCmd.Flags().StringVarP(&costProvider, "provider", "p", "", "filter by provider")
	rootCmd.AddCommand(costCmd)
}
