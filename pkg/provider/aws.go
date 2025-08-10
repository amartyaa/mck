package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// AWSProvider implements Provider for Amazon EKS.
type AWSProvider struct {
	profile string
	regions []string
}

// NewAWSProvider creates an AWS provider from config.
func NewAWSProvider(profile string, regions []string) *AWSProvider {
	return &AWSProvider{
		profile: profile,
		regions: regions,
	}
}

func (a *AWSProvider) Name() string { return "aws" }

func (a *AWSProvider) newEKSClient(ctx context.Context, region string) (*eks.Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if a.profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(a.profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for region %s: %w", region, err)
	}
	return eks.NewFromConfig(cfg), nil
}

func (a *AWSProvider) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	var all []ClusterInfo

	for _, region := range a.regions {
		client, err := a.newEKSClient(ctx, region)
		if err != nil {
			return nil, err
		}

		paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("listing EKS clusters in %s: %w", region, err)
			}

			for _, name := range page.Clusters {
				info, err := a.describeCluster(ctx, client, name, region)
				if err != nil {
					// Log warning but continue — one cluster failing shouldn't break the list
					info = &ClusterInfo{
						Name:     name,
						Provider: "aws",
						Region:   region,
						Status:   "ERROR",
					}
				}
				all = append(all, *info)
			}
		}
	}

	return all, nil
}

func (a *AWSProvider) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	for _, region := range a.regions {
		client, err := a.newEKSClient(ctx, region)
		if err != nil {
			continue
		}
		info, err := a.describeCluster(ctx, client, name, region)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("cluster %q not found in any configured AWS region", name)
}

func (a *AWSProvider) describeCluster(ctx context.Context, client *eks.Client, name, region string) (*ClusterInfo, error) {
	out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &name,
	})
	if err != nil {
		return nil, err
	}

	c := out.Cluster
	status := "UNKNOWN"
	if c.Status != "" {
		status = string(c.Status)
	}

	return &ClusterInfo{
		Name:       *c.Name,
		Provider:   "aws",
		Region:     region,
		Version:    deref(c.Version),
		Status:     status,
		Endpoint:   deref(c.Endpoint),
		ResourceID: deref(c.Arn),
	}, nil
}

func (a *AWSProvider) GetKubeconfig(ctx context.Context, clusterName string) ([]byte, error) {
	cluster, err := a.GetCluster(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// Get the cluster's CA certificate
	for _, region := range a.regions {
		client, err := a.newEKSClient(ctx, region)
		if err != nil {
			continue
		}
		out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: &clusterName,
		})
		if err != nil {
			continue
		}

		caData := ""
		if out.Cluster.CertificateAuthority != nil {
			caData = deref(out.Cluster.CertificateAuthority.Data)
		}

		kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    certificate-authority-data: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - %s
        - --region
        - %s
`,
			cluster.Endpoint, caData,
			clusterName, clusterName, clusterName, clusterName, clusterName,
			clusterName, clusterName, region,
		)
		return []byte(kubeconfig), nil
	}

	return nil, fmt.Errorf("could not generate kubeconfig for %s", clusterName)
}

func (a *AWSProvider) GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error) {
	for _, region := range a.regions {
		client, err := a.newEKSClient(ctx, region)
		if err != nil {
			continue
		}

		// List nodegroups to get instance types and counts
		ngOut, err := client.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: &clusterName,
		})
		if err != nil {
			continue
		}

		var nodes []NodeInfo
		for _, ngName := range ngOut.Nodegroups {
			ng, err := client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   &clusterName,
				NodegroupName: &ngName,
			})
			if err != nil {
				continue
			}

			instanceType := "unknown"
			if len(ng.Nodegroup.InstanceTypes) > 0 {
				instanceType = ng.Nodegroup.InstanceTypes[0]
			}

			desiredCount := 0
			if ng.Nodegroup.ScalingConfig != nil && ng.Nodegroup.ScalingConfig.DesiredSize != nil {
				desiredCount = int(*ng.Nodegroup.ScalingConfig.DesiredSize)
			}

			for i := 0; i < desiredCount; i++ {
				nodes = append(nodes, NodeInfo{
					Name:         fmt.Sprintf("%s-node-%d", ngName, i),
					InstanceType: instanceType,
					Status:       string(ng.Nodegroup.Status),
				})
			}
		}
		return nodes, nil
	}

	return nil, fmt.Errorf("cluster %q not found", clusterName)
}

func (a *AWSProvider) GetCost(ctx context.Context, clusterName string, period string) (*CostInfo, error) {
	region := "us-east-1" // Cost Explorer is global, but client needs a region
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if a.profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(a.profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for Cost Explorer: %w", err)
	}

	ceClient := costexplorer.NewFromConfig(cfg)

	// Period format: "2025-09" -> start "2025-09-01", end "2025-10-01"
	startDate := period + "-01"
	endDate := nextMonth(period) + "-01"

	out, err := ceClient.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: &startDate,
			End:   &endDate,
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter: &types.Expression{
			Tags: &types.TagValues{
				Key:    strPtr("kubernetes.io/cluster/" + clusterName),
				Values: []string{"owned"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting cost data: %w", err)
	}

	totalCost := 0.0
	for _, result := range out.ResultsByTime {
		if amount, ok := result.Total["UnblendedCost"]; ok && amount.Amount != nil {
			totalCost += parseFloat(deref(amount.Amount))
		}
	}

	return &CostInfo{
		ClusterName:     clusterName,
		Provider:        "aws",
		MonthlyEstimate: totalCost,
		Currency:        "USD",
		Period:          period,
	}, nil
}

// helper functions
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string { return &s }

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func nextMonth(period string) string {
	var year, month int
	fmt.Sscanf(period, "%d-%d", &year, &month)
	month++
	if month > 12 {
		month = 1
		year++
	}
	return fmt.Sprintf("%d-%02d", year, month)
}

// base64 decode helper (used internally)
var _ = base64.StdEncoding
