package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/amartyaa/mck/pkg/kubeconfig"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	applyClusters string
	applyFile     string
	applyDryRun   bool
	applyParallel bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply Kubernetes manifests to one or more clusters",
	Long: `Deploy manifests to multiple clusters in a single command.
Clusters can be specified as a comma-separated list of names or aliases.

Examples:
  mck apply -f deployment.yaml --clusters prod-eks,prod-aks
  mck apply -f ./manifests/ --clusters staging-gke --dry-run
  mck apply -f service.yaml --clusters prod,staging --parallel`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if applyFile == "" {
			return fmt.Errorf("--file (-f) is required")
		}
		if applyClusters == "" {
			return fmt.Errorf("--clusters is required")
		}

		clusterNames := parseClusterList(applyClusters)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		registry := initProviders()

		if applyParallel {
			return applyParallelMode(ctx, registry, clusterNames)
		}
		return applySequentialMode(ctx, registry, clusterNames)
	},
}

func applySequentialMode(ctx context.Context, registry interface{}, clusterNames []string) error {
	for _, name := range clusterNames {
		name = resolveAlias(name)
		fmt.Printf("\n📦 Deploying to %s...\n", name)

		if err := applyToCluster(ctx, name); err != nil {
			color.Red("✗ %s: %v", name, err)
			return fmt.Errorf("deployment to %s failed, aborting", name)
		}
		color.Green("✓ %s: deployed successfully", name)
	}
	return nil
}

func applyParallelMode(ctx context.Context, registry interface{}, clusterNames []string) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(clusterNames))

	for _, name := range clusterNames {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			clusterName = resolveAlias(clusterName)
			fmt.Printf("📦 Deploying to %s...\n", clusterName)

			if err := applyToCluster(ctx, clusterName); err != nil {
				color.Red("✗ %s: %v", clusterName, err)
				errors <- fmt.Errorf("%s: %w", clusterName, err)
				return
			}
			color.Green("✓ %s: deployed successfully", clusterName)
		}(name)
	}

	wg.Wait()
	close(errors)

	var errs []string
	for err := range errors {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("deployment failed for: %s", strings.Join(errs, "; "))
	}
	return nil
}

func applyToCluster(ctx context.Context, clusterName string) error {
	// Save kubeconfig to a temp file for this specific apply
	tmpFile := fmt.Sprintf("%s/mck-apply-%s", os.TempDir(), clusterName)
	defer os.Remove(tmpFile)

	// First, get the kubeconfig for this cluster
	reg := initProviders()
	for _, p := range reg.All() {
		kc, err := p.GetKubeconfig(ctx, clusterName)
		if err != nil {
			continue
		}
		if err := kubeconfig.SaveToFile(kc, tmpFile); err != nil {
			return fmt.Errorf("saving temp kubeconfig: %w", err)
		}

		// Run kubectl apply
		args := []string{"apply", "-f", applyFile}
		if applyDryRun {
			args = append(args, "--dry-run=client")
		}

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+tmpFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	return fmt.Errorf("cluster %q not found", clusterName)
}

func parseClusterList(s string) []string {
	var result []string
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "path to manifest file or directory")
	applyCmd.Flags().StringVar(&applyClusters, "clusters", "", "comma-separated list of cluster names")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "run kubectl apply with --dry-run=client")
	applyCmd.Flags().BoolVar(&applyParallel, "parallel", false, "deploy to all clusters in parallel")
	rootCmd.AddCommand(applyCmd)
}
