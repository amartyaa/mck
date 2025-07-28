package provider

import "context"

// ClusterInfo holds metadata about a Kubernetes cluster from any cloud provider.
type ClusterInfo struct {
	Name       string            `json:"name" yaml:"name"`
	Provider   string            `json:"provider" yaml:"provider"`     // aws, gcp, azure, oci
	Region     string            `json:"region" yaml:"region"`
	Version    string            `json:"version" yaml:"version"`
	Status     string            `json:"status" yaml:"status"`
	NodeCount  int               `json:"node_count" yaml:"node_count"`
	Endpoint   string            `json:"endpoint" yaml:"endpoint"`
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Account    string            `json:"account" yaml:"account"`       // account/project/subscription ID
	ResourceID string            `json:"resource_id" yaml:"resource_id"`
}

// NodeInfo holds metadata about a node in a cluster.
type NodeInfo struct {
	Name         string `json:"name" yaml:"name"`
	InstanceType string `json:"instance_type" yaml:"instance_type"`
	Zone         string `json:"zone" yaml:"zone"`
	Status       string `json:"status" yaml:"status"`
	CPUCapacity  string `json:"cpu_capacity" yaml:"cpu_capacity"`
	MemCapacity  string `json:"mem_capacity" yaml:"mem_capacity"`
	CPUUsed      string `json:"cpu_used" yaml:"cpu_used"`
	MemUsed      string `json:"mem_used" yaml:"mem_used"`
	Pods         int    `json:"pods" yaml:"pods"`
}

// CostInfo holds cost data for a cluster.
type CostInfo struct {
	ClusterName   string  `json:"cluster_name" yaml:"cluster_name"`
	Provider      string  `json:"provider" yaml:"provider"`
	Region        string  `json:"region" yaml:"region"`
	MonthlyEstimate float64 `json:"monthly_estimate" yaml:"monthly_estimate"`
	Currency      string  `json:"currency" yaml:"currency"`
	ComputeCost   float64 `json:"compute_cost" yaml:"compute_cost"`
	NetworkCost   float64 `json:"network_cost" yaml:"network_cost"`
	StorageCost   float64 `json:"storage_cost" yaml:"storage_cost"`
	Period        string  `json:"period" yaml:"period"` // e.g., "2025-09"
}

// Provider is the interface every cloud provider must implement.
// This is the core abstraction — AWS, GCP, Azure, OCI all satisfy this contract.
type Provider interface {
	// Name returns the provider identifier (e.g., "aws", "gcp", "azure", "oci").
	Name() string

	// ListClusters returns all Kubernetes clusters accessible with current credentials.
	ListClusters(ctx context.Context) ([]ClusterInfo, error)

	// GetCluster returns details for a specific cluster by name.
	GetCluster(ctx context.Context, name string) (*ClusterInfo, error)

	// GetKubeconfig returns kubeconfig YAML bytes for the specified cluster.
	// This is the core of the `connect` command.
	GetKubeconfig(ctx context.Context, clusterName string) ([]byte, error)

	// GetNodes returns node-level status for a cluster.
	GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error)

	// GetCost returns cost info for a cluster over a given period.
	GetCost(ctx context.Context, clusterName string, period string) (*CostInfo, error)
}
