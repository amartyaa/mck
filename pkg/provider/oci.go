package provider

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
)

// OCIProvider implements Provider for Oracle Container Engine for Kubernetes (OKE).
type OCIProvider struct {
	compartments []string
	profile      string
}

// NewOCIProvider creates an OCI provider from config.
func NewOCIProvider(compartments []string, profile string) *OCIProvider {
	return &OCIProvider{
		compartments: compartments,
		profile:      profile,
	}
}

func (o *OCIProvider) Name() string { return "oci" }

func (o *OCIProvider) newClient() (containerengine.ContainerEngineClient, error) {
	var configProvider common.ConfigurationProvider

	if o.profile != "" && o.profile != "DEFAULT" {
		configProvider = common.CustomProfileConfigProvider("~/.oci/config", o.profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	client, err := containerengine.NewContainerEngineClientWithConfigurationProvider(configProvider)
	if err != nil {
		return containerengine.ContainerEngineClient{}, fmt.Errorf("creating OKE client: %w", err)
	}
	return client, nil
}

func (o *OCIProvider) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	client, err := o.newClient()
	if err != nil {
		return nil, err
	}

	var all []ClusterInfo

	for _, compartmentID := range o.compartments {
		req := containerengine.ListClustersRequest{
			CompartmentId: &compartmentID,
		}

		resp, err := client.ListClusters(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("listing OKE clusters in compartment %s: %w", compartmentID, err)
		}

		for _, c := range resp.Items {
			nodeCount := 0

			// Get node pools for this cluster
			npReq := containerengine.ListNodePoolsRequest{
				CompartmentId: &compartmentID,
				ClusterId:     c.Id,
			}
			npResp, err := client.ListNodePools(ctx, npReq)
			if err == nil {
				for _, np := range npResp.Items {
					if np.NodeConfigDetails != nil && np.NodeConfigDetails.Size != nil {
						nodeCount += int(*np.NodeConfigDetails.Size)
					}
				}
			}

			endpoint := ""
			if c.Endpoints != nil && c.Endpoints.Kubernetes != nil {
				endpoint = *c.Endpoints.Kubernetes
			}

			version := ""
			if c.KubernetesVersion != nil {
				version = *c.KubernetesVersion
			}

			all = append(all, ClusterInfo{
				Name:       deref(c.Name),
				Provider:   "oci",
				Region:     extractOCIRegion(deref(c.Id)),
				Version:    version,
				Status:     string(c.LifecycleState),
				NodeCount:  nodeCount,
				Endpoint:   endpoint,
				Account:    compartmentID,
				ResourceID: deref(c.Id),
				Labels:     c.FreeformTags,
			})
		}
	}

	return all, nil
}

func (o *OCIProvider) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	clusters, err := o.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, fmt.Errorf("OKE cluster %q not found in any configured compartment", name)
}

func (o *OCIProvider) GetKubeconfig(ctx context.Context, clusterName string) ([]byte, error) {
	client, err := o.newClient()
	if err != nil {
		return nil, err
	}

	// Find the cluster ID first
	cluster, err := o.GetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	req := containerengine.CreateKubeconfigRequest{
		ClusterId: &cluster.ResourceID,
	}

	resp, err := client.CreateKubeconfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("generating OKE kubeconfig for %s: %w", clusterName, err)
	}

	// Read the response body
	buf := make([]byte, 0)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Content.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}

	return buf, nil
}

func (o *OCIProvider) GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error) {
	client, err := o.newClient()
	if err != nil {
		return nil, err
	}

	cluster, err := o.GetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	compartmentID := cluster.Account
	clusterID := cluster.ResourceID

	npReq := containerengine.ListNodePoolsRequest{
		CompartmentId: &compartmentID,
		ClusterId:     &clusterID,
	}
	npResp, err := client.ListNodePools(ctx, npReq)
	if err != nil {
		return nil, fmt.Errorf("listing OKE node pools: %w", err)
	}

	var nodes []NodeInfo
	for _, np := range npResp.Items {
		shape := ""
		if np.NodeShape != nil {
			shape = *np.NodeShape
		}

		count := 0
		if np.NodeConfigDetails != nil && np.NodeConfigDetails.Size != nil {
			count = int(*np.NodeConfigDetails.Size)
		}

		for i := 0; i < count; i++ {
			nodes = append(nodes, NodeInfo{
				Name:         fmt.Sprintf("%s-node-%d", deref(np.Name), i),
				InstanceType: shape,
				Status:       string(np.LifecycleState),
			})
		}
	}

	return nodes, nil
}

func (o *OCIProvider) GetCost(ctx context.Context, clusterName string, period string) (*CostInfo, error) {
	// OCI cost data requires the Usage API
	return &CostInfo{
		ClusterName:     clusterName,
		Provider:        "oci",
		MonthlyEstimate: 0,
		Currency:        "USD",
		Period:          period,
	}, fmt.Errorf("OCI cost retrieval requires Usage API setup — see docs")
}

// helpers

func extractOCIRegion(ocid string) string {
	// OCID format: ocid1.cluster.oc1.<region>.<unique_id>
	parts := splitOCID(ocid)
	if len(parts) >= 4 {
		return parts[3]
	}
	return "unknown"
}

func splitOCID(s string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
