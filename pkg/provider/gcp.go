package provider

import (
	"context"
	"fmt"
	"strings"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	billing "cloud.google.com/go/billing/budgets/apiv1"
	"google.golang.org/api/iterator"
)

// GCPProvider implements Provider for Google Kubernetes Engine (GKE).
type GCPProvider struct {
	projects []string
	zones    []string // empty = all zones
}

// NewGCPProvider creates a GCP provider from config.
func NewGCPProvider(projects []string, zones []string) *GCPProvider {
	return &GCPProvider{
		projects: projects,
		zones:    zones,
	}
}

func (g *GCPProvider) Name() string { return "gcp" }

func (g *GCPProvider) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	client, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GKE client: %w", err)
	}
	defer client.Close()

	var all []ClusterInfo

	for _, project := range g.projects {
		// "-" means all zones/regions
		parent := fmt.Sprintf("projects/%s/locations/-", project)

		resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
			Parent: parent,
		})
		if err != nil {
			return nil, fmt.Errorf("listing GKE clusters in project %s: %w", project, err)
		}

		for _, c := range resp.Clusters {
			// If specific zones are configured, filter
			if len(g.zones) > 0 && !containsZone(g.zones, c.Location) {
				continue
			}

			all = append(all, ClusterInfo{
				Name:       c.Name,
				Provider:   "gcp",
				Region:     c.Location,
				Version:    c.CurrentMasterVersion,
				Status:     c.Status.String(),
				NodeCount:  countGKENodes(c),
				Endpoint:   c.Endpoint,
				Account:    project,
				ResourceID: c.SelfLink,
				Labels:     c.ResourceLabels,
			})
		}
	}

	return all, nil
}

func (g *GCPProvider) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	client, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GKE client: %w", err)
	}
	defer client.Close()

	for _, project := range g.projects {
		parent := fmt.Sprintf("projects/%s/locations/-", project)
		resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
			Parent: parent,
		})
		if err != nil {
			continue
		}

		for _, c := range resp.Clusters {
			if c.Name == name {
				return &ClusterInfo{
					Name:       c.Name,
					Provider:   "gcp",
					Region:     c.Location,
					Version:    c.CurrentMasterVersion,
					Status:     c.Status.String(),
					NodeCount:  countGKENodes(c),
					Endpoint:   c.Endpoint,
					Account:    project,
					ResourceID: c.SelfLink,
					Labels:     c.ResourceLabels,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("GKE cluster %q not found in any configured project", name)
}

func (g *GCPProvider) GetKubeconfig(ctx context.Context, clusterName string) ([]byte, error) {
	cluster, err := g.GetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// GKE kubeconfigs use gke-gcloud-auth-plugin for authentication
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://%s
  name: gke_%s_%s_%s
contexts:
- context:
    cluster: gke_%s_%s_%s
    user: gke_%s_%s_%s
  name: gke_%s_%s_%s
current-context: gke_%s_%s_%s
users:
- name: gke_%s_%s_%s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: gke-gcloud-auth-plugin
      installHint: Install gke-gcloud-auth-plugin for use with kubectl by following https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl#install_plugin
      provideClusterInfo: true
`,
		cluster.Endpoint,
		cluster.Account, cluster.Region, clusterName,
		cluster.Account, cluster.Region, clusterName,
		cluster.Account, cluster.Region, clusterName,
		cluster.Account, cluster.Region, clusterName,
		cluster.Account, cluster.Region, clusterName,
		cluster.Account, cluster.Region, clusterName,
	)

	return []byte(kubeconfig), nil
}

func (g *GCPProvider) GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error) {
	client, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating GKE client: %w", err)
	}
	defer client.Close()

	for _, project := range g.projects {
		parent := fmt.Sprintf("projects/%s/locations/-", project)
		resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
			Parent: parent,
		})
		if err != nil {
			continue
		}

		for _, c := range resp.Clusters {
			if c.Name != clusterName {
				continue
			}

			var nodes []NodeInfo
			for _, np := range c.NodePools {
				count := int(np.InitialNodeCount)
				if np.Autoscaling != nil && np.Autoscaling.Enabled {
					count = int(np.Autoscaling.MinNodeCount)
				}

				for i := 0; i < count; i++ {
					nodes = append(nodes, NodeInfo{
						Name:         fmt.Sprintf("%s-node-%d", np.Name, i),
						InstanceType: np.Config.MachineType,
						Status:       np.Status.String(),
						CPUCapacity:  fmt.Sprintf("%d", np.Config.DiskSizeGb),
					})
				}
			}
			return nodes, nil
		}
	}

	return nil, fmt.Errorf("GKE cluster %q not found", clusterName)
}

func (g *GCPProvider) GetCost(ctx context.Context, clusterName string, period string) (*CostInfo, error) {
	// GCP cost data requires BigQuery export or Billing API
	// This is a simplified implementation using the billing budgets API
	_ = billing.NewBudgetClient
	_ = iterator.Done

	return &CostInfo{
		ClusterName:     clusterName,
		Provider:        "gcp",
		MonthlyEstimate: 0,
		Currency:        "USD",
		Period:          period,
	}, fmt.Errorf("GCP cost retrieval requires BigQuery billing export setup — see docs")
}

// helpers

func countGKENodes(c *containerpb.Cluster) int {
	total := 0
	for _, np := range c.NodePools {
		total += int(np.InitialNodeCount)
	}
	return total
}

func containsZone(zones []string, zone string) bool {
	for _, z := range zones {
		if z == zone || strings.HasPrefix(zone, z) {
			return true
		}
	}
	return false
}
