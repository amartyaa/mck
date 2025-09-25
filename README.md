# mck — Multi-Cloud Kubernetes CLI

> One tool. Every cloud. All your clusters.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**mck** is a unified command-line tool for managing Kubernetes clusters across AWS EKS, Azure AKS, Google GKE, and Oracle OKE — from a single interface.

## Why?

Managing Kubernetes across multiple clouds means juggling `aws`, `gcloud`, `az`, and `oci` CLIs, each with different auth flows, naming conventions, and kubeconfig formats. `mck` unifies all of this into one tool:

```bash
# Before: four CLIs, four auth flows
aws eks update-kubeconfig --name prod --region us-east-1
gcloud container clusters get-credentials staging --zone us-central1
az aks get-credentials --resource-group rg --name dev
oci ce cluster create-kubeconfig --cluster-id ocid1...

# After: one command
mck connect prod-eks
mck connect staging-gke
mck connect dev-aks
```

## Installation

```bash
# Go install
go install github.com/amartyaa/mck@latest

# Or download pre-built binaries from Releases
```

## Quick Start

### 1. Create a config file

```bash
cp configs/mck.yaml.example ~/.mck.yaml
# Edit with your cloud provider details
```

### 2. List all clusters

```bash
$ mck list

NAME                    PROVIDER  REGION          VERSION   STATUS    NODES
────────────────────────────────────────────────────────────────────────────
prod-cluster            aws       us-east-1       1.29      ACTIVE    6
staging-gke             gcp       us-central1     1.29      RUNNING   3
dev-aks                 azure     centralindia    1.29      Succeeded 2
test-oke                oci       ap-mumbai-1     1.28      ACTIVE    2
```

### 3. Connect to a cluster

```bash
$ mck connect prod-cluster
✓ Connected to prod-cluster (aws) — kubectl is ready
```

### 4. Check status

```bash
$ mck status prod-cluster

Cluster: prod-cluster (aws / us-east-1)
Version: 1.29   Status: ACTIVE   Nodes: 6

NODE                  INSTANCE TYPE     ZONE          STATUS   CPU        MEMORY
──────────────────────────────────────────────────────────────────────────────────
system-node-0         m5.xlarge         us-east-1a    Ready    2/4        4Gi/16Gi
system-node-1         m5.xlarge         us-east-1b    Ready    1/4        3Gi/16Gi
general-node-0        m5.2xlarge        us-east-1a    Ready    6/8        12Gi/32Gi
...
```

### 5. Deploy across clusters

```bash
$ mck apply -f deployment.yaml --clusters prod-eks,prod-aks --parallel

📦 Deploying to prod-eks...
📦 Deploying to prod-aks...
✓ prod-eks: deployed successfully
✓ prod-aks: deployed successfully
```

### 6. Cost overview

```bash
$ mck cost --month 2025-08

CLUSTER              PROVIDER  REGION          MONTHLY COST  CURRENCY
─────────────────────────────────────────────────────────────────────
prod-cluster         aws       us-east-1       1,247.30      USD
staging-gke          gcp       us-central1       423.50      USD
dev-aks              azure     centralindia      189.00      USD
                               TOTAL           1,859.80      USD
```

## Commands

| Command | Description |
|---------|-------------|
| `mck list` | List clusters across all providers |
| `mck connect <name>` | Fetch kubeconfig and set kubectl context |
| `mck status [name]` | Show node/resource status |
| `mck apply -f <file>` | Deploy to one or more clusters |
| `mck cost` | View cost data across providers |
| `mck version` | Print version info |

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `~/.mck.yaml`) |
| `-o, --output` | Output format: `table`, `json`, `yaml` |

## Configuration

mck reads from `~/.mck.yaml`. See [configs/mck.yaml.example](configs/mck.yaml.example) for all options.

```yaml
providers:
  aws:
    profile: default
    regions: [us-east-1, eu-west-1]
  gcp:
    projects: [my-project]
  azure:
    subscription_ids: ["00000000-..."]
  oci:
    compartments: ["ocid1.compartment..."]

aliases:
  prod: "aws:us-east-1:prod-cluster"
  staging: "gcp:us-central1:staging-gke"
```

## Architecture

```
mck
├── cmd/               # Cobra commands
│   ├── root.go        # CLI entry point + config loading
│   ├── list.go        # List clusters across providers
│   ├── connect.go     # Kubeconfig fetch + context switch
│   ├── status.go      # Node-level status
│   ├── apply.go       # Multi-cluster deployment
│   ├── cost.go        # Cost aggregation
│   └── version.go     # Build info
├── pkg/
│   ├── provider/      # Cloud provider abstraction
│   │   ├── provider.go    # Interface definition
│   │   ├── registry.go    # Provider registry
│   │   ├── aws.go         # Amazon EKS
│   │   ├── gcp.go         # Google GKE
│   │   ├── azure.go       # Azure AKS
│   │   └── oci.go         # Oracle OKE
│   ├── kubeconfig/    # Kubeconfig management
│   ├── config/        # YAML config handling
│   └── output/        # Table/JSON/YAML formatters
├── configs/           # Example configs
├── .goreleaser.yaml   # Cross-platform release builds
└── go.mod
```

## Auth Prerequisites

Each provider uses its native SDK authentication:

| Provider | Auth Method |
|----------|-------------|
| **AWS** | AWS CLI profile, env vars, or IAM role |
| **GCP** | `gcloud auth application-default login` or service account JSON |
| **Azure** | `az login`, managed identity, or service principal |
| **OCI** | `~/.oci/config` or instance principal |

## Building from Source

```bash
git clone https://github.com/amartyaa/mck.git
cd mck
go build -o mck .

# With version info
go build -ldflags "-X github.com/amartyaa/mck/cmd.Version=v0.1.0" -o mck .
```

## License

MIT — [Amartya Anshuman](https://amartya.is-a.dev)
