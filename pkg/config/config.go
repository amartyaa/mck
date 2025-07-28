package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the mck configuration file structure.
type Config struct {
	// DefaultProvider is the provider to use when --provider flag is not specified.
	DefaultProvider string `yaml:"default_provider,omitempty"`

	// DefaultOutput is the output format (table, json, yaml).
	DefaultOutput string `yaml:"default_output,omitempty"`

	// Providers holds per-provider configuration.
	Providers ProvidersConfig `yaml:"providers"`

	// Aliases maps friendly names to cluster identifiers.
	// e.g., "prod" -> "aws:us-east-1:my-prod-cluster"
	Aliases map[string]string `yaml:"aliases,omitempty"`
}

// ProvidersConfig holds configuration for each cloud provider.
type ProvidersConfig struct {
	AWS   *AWSConfig   `yaml:"aws,omitempty"`
	GCP   *GCPConfig   `yaml:"gcp,omitempty"`
	Azure *AzureConfig `yaml:"azure,omitempty"`
	OCI   *OCIConfig   `yaml:"oci,omitempty"`
}

// AWSConfig holds AWS-specific configuration.
type AWSConfig struct {
	Profile string   `yaml:"profile,omitempty"` // AWS CLI profile
	Regions []string `yaml:"regions"`           // Regions to scan for clusters
}

// GCPConfig holds GCP-specific configuration.
type GCPConfig struct {
	Projects []string `yaml:"projects"`          // GCP projects to scan
	Zones    []string `yaml:"zones,omitempty"`   // Specific zones (optional, defaults to all)
}

// AzureConfig holds Azure-specific configuration.
type AzureConfig struct {
	SubscriptionIDs []string `yaml:"subscription_ids"` // Azure subscriptions to scan
	ResourceGroups  []string `yaml:"resource_groups,omitempty"`
}

// OCIConfig holds Oracle Cloud Infrastructure configuration.
type OCIConfig struct {
	Compartments []string `yaml:"compartments"` // OCI compartment OCIDs
	Profile      string   `yaml:"profile,omitempty"`
}

// Load reads the config from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}

// DefaultPath returns the default config path: ~/.mck.yaml
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mck.yaml"
	}
	return filepath.Join(home, ".mck.yaml")
}

// Save writes the config to the given path.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
