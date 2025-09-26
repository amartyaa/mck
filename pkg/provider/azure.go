package provider

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
)

// AzureProvider implements Provider for Azure Kubernetes Service (AKS).
type AzureProvider struct {
	subscriptionIDs []string
	resourceGroups  []string // empty = scan all RGs
}

// NewAzureProvider creates an Azure provider from config.
func NewAzureProvider(subscriptionIDs []string, resourceGroups []string) *AzureProvider {
	return &AzureProvider{
		subscriptionIDs: subscriptionIDs,
		resourceGroups:  resourceGroups,
	}
}

func (az *AzureProvider) Name() string { return "azure" }

func (az *AzureProvider) newClient(subscriptionID string) (*armcontainerservice.ManagedClustersClient, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure authentication: %w", err)
	}
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("creating AKS client: %w", err)
	}
	return client, nil
}

func (az *AzureProvider) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	var all []ClusterInfo

	for _, subID := range az.subscriptionIDs {
		client, err := az.newClient(subID)
		if err != nil {
			return nil, err
		}

		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("listing AKS clusters in subscription %s: %w", subID, err)
			}

			for _, c := range page.Value {
				// If specific resource groups are configured, filter
				if len(az.resourceGroups) > 0 && !containsString(az.resourceGroups, extractRG(*c.ID)) {
					continue
				}

				nodeCount := 0
				if c.Properties != nil && c.Properties.AgentPoolProfiles != nil {
					for _, pool := range c.Properties.AgentPoolProfiles {
						if pool.Count != nil {
							nodeCount += int(*pool.Count)
						}
					}
				}

				version := ""
				if c.Properties != nil && c.Properties.KubernetesVersion != nil {
					version = *c.Properties.KubernetesVersion
				}

				status := "UNKNOWN"
				if c.Properties != nil && c.Properties.ProvisioningState != nil {
					status = *c.Properties.ProvisioningState
				}

				endpoint := ""
				if c.Properties != nil && c.Properties.Fqdn != nil {
					endpoint = *c.Properties.Fqdn
				}

				all = append(all, ClusterInfo{
					Name:       deref(c.Name),
					Provider:   "azure",
					Region:     deref(c.Location),
					Version:    version,
					Status:     status,
					NodeCount:  nodeCount,
					Endpoint:   endpoint,
					Account:    subID,
					ResourceID: deref(c.ID),
					Labels:     toStringMap(c.Tags),
				})
			}
		}
	}

	return all, nil
}

func (az *AzureProvider) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	clusters, err := az.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("AKS cluster %q not found in any configured subscription", name)
}

func (az *AzureProvider) GetKubeconfig(ctx context.Context, clusterName string) ([]byte, error) {
	for _, subID := range az.subscriptionIDs {
		client, err := az.newClient(subID)
		if err != nil {
			continue
		}

		// Find the resource group for this cluster
		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				break
			}
			for _, c := range page.Value {
				if deref(c.Name) != clusterName {
					continue
				}
				resourceGroup := extractRG(deref(c.ID))

				// Get admin credentials (kubeconfig)
				result, err := client.ListClusterAdminCredentials(ctx, resourceGroup, clusterName, nil)
				if err != nil {
					return nil, fmt.Errorf("getting AKS kubeconfig: %w", err)
				}

				if len(result.Kubeconfigs) > 0 && result.Kubeconfigs[0].Value != nil {
					return result.Kubeconfigs[0].Value, nil
				}
				return nil, fmt.Errorf("empty kubeconfig returned for %s", clusterName)
			}
		}
	}

	return nil, fmt.Errorf("AKS cluster %q not found", clusterName)
}

func (az *AzureProvider) GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error) {
	for _, subID := range az.subscriptionIDs {
		client, err := az.newClient(subID)
		if err != nil {
			continue
		}

		pager := client.NewListPager(nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				break
			}
			for _, c := range page.Value {
				if deref(c.Name) != clusterName || c.Properties == nil {
					continue
				}

				var nodes []NodeInfo
				for _, pool := range c.Properties.AgentPoolProfiles {
					count := 0
					if pool.Count != nil {
						count = int(*pool.Count)
					}
					vmSize := ""
					if pool.VMSize != nil {
						vmSize = *pool.VMSize
					}

					for i := 0; i < count; i++ {
						nodes = append(nodes, NodeInfo{
							Name:         fmt.Sprintf("%s-node-%d", deref(pool.Name), i),
							InstanceType: vmSize,
							Status:       string(*pool.ProvisioningState),
						})
					}
				}
				return nodes, nil
			}
		}
	}

	return nil, fmt.Errorf("AKS cluster %q not found", clusterName)
}

func (az *AzureProvider) GetCost(ctx context.Context, clusterName string, period string) (*CostInfo, error) {
	// Azure cost analysis requires the Consumption/Cost Management API
	// Full implementation would use armcostmanagement.QueryClient
	return &CostInfo{
		ClusterName:     clusterName,
		Provider:        "azure",
		MonthlyEstimate: 0,
		Currency:        "USD",
		Period:          period,
	}, fmt.Errorf("Azure cost retrieval requires Cost Management API setup — see docs")
}

// helpers

func extractRG(resourceID string) string {
	// /subscriptions/.../resourceGroups/MY-RG/providers/...
	parts := splitPath(resourceID)
	for i, p := range parts {
		if eqFold(p, "resourceGroups") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func splitPath(s string) []string {
	var parts []string
	for _, p := range split(s, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func split(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func eqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func toStringMap(m map[string]*string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if eqFold(item, s) {
			return true
		}
	}
	return false
}
