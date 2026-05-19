# GCP Runtime Plane — Terraform

This module provisions a fully-featured GCP **Runtime Plane** for the cloud-agnostic PaaS platform.

## What gets created

| Resource | Details |
|---|---|
| **VPC** | Custom-mode VPC (`platform-vpc`) with a single regional subnet (10.0.0.0/16) and secondary ranges for GKE pods (10.100.0.0/14) and services (10.104.0.0/20) |
| **Cloud NAT** | Cloud Router + NAT gateway so private nodes can reach the internet without public IPs |
| **GKE cluster** | Private Standard cluster (REGULAR release channel), Workload Identity, Calico network policy, Cloud Operations logging/monitoring |
| **Node pool** | Autoscaling node pool (e2-standard-2 by default); nodes have no external IPs |
| **Artifact Registry** | Docker repository `platform-apps` in the same region; GKE nodes have read access |
| **IAM** | Minimal node SA; dedicated runtime-agent SA with `artifactregistry.writer` + `container.developer`; Workload Identity binding |
| **Runtime agent** | Kubernetes `Deployment` in `platform-system` namespace wired to the control plane |
| **nginx ingress** | `ingress-nginx` Helm release with a GCP LoadBalancer |

## Prerequisites

- Terraform >= 1.5
- GCP project with the following APIs enabled:
  - `container.googleapis.com`
  - `artifactregistry.googleapis.com`
  - `compute.googleapis.com`
  - `iam.googleapis.com`
- A service account or user identity with `roles/owner` or an equivalent custom role covering the resources above
- `gcloud` CLI installed and authenticated (`gcloud auth application-default login`)

## Quick start

```bash
cd infra/gcp

# 1. Initialise providers
terraform init

# 2. Preview the changes
terraform plan \
  -var="project_id=my-gcp-project" \
  -var="control_plane_url=https://control-plane.example.com"

# 3. Apply
terraform apply \
  -var="project_id=my-gcp-project" \
  -var="control_plane_url=https://control-plane.example.com"
```

### Optional variables

| Variable | Default | Description |
|---|---|---|
| `region` | `us-central1` | GCP region |
| `cluster_name` | `platform-runtime` | GKE cluster name |
| `runtime_name` | `gcp-us-central1` | Runtime plane identifier sent to the control plane |
| `kubernetes_version` | `1.28` | Minimum GKE master version |
| `node_machine_type` | `e2-standard-2` | GCE machine type for worker nodes |
| `node_min_count` | `1` | Autoscaler minimum |
| `node_max_count` | `5` | Autoscaler maximum |
| `node_initial_count` | `2` | Nodes at creation time |
| `environment` | `production` | Environment label |
| `master_authorized_cidr` | `0.0.0.0/0` | CIDR allowed to reach the API server |

## Using a tfvars file

```hcl
# terraform.tfvars
project_id        = "my-gcp-project"
region            = "us-central1"
control_plane_url = "https://control-plane.example.com"
environment       = "production"
node_max_count    = 10
```

```bash
terraform apply -var-file=terraform.tfvars
```

## Connecting kubectl

After a successful apply, run the command printed in the `kubeconfig_command` output:

```bash
terraform output -raw kubeconfig_command | bash
# equivalent to:
# gcloud container clusters get-credentials platform-runtime \
#   --region us-central1 --project my-gcp-project
```

Verify access:

```bash
kubectl get nodes
kubectl -n platform-system get pods
```

## Key outputs

| Output | Description |
|---|---|
| `cluster_name` | GKE cluster name |
| `cluster_endpoint` | API server IP (sensitive) |
| `registry_url` | Artifact Registry base URL for pushing/pulling images |
| `kubeconfig_command` | `gcloud` command to configure kubectl |
| `workload_identity_sa_email` | GCP SA email used by the runtime agent |

## Pushing images

```bash
# Authenticate Docker to Artifact Registry
gcloud auth configure-docker $(terraform output -raw registry_url | cut -d/ -f1)

# Tag and push
docker tag my-app:latest $(terraform output -raw registry_url)/my-app:latest
docker push $(terraform output -raw registry_url)/my-app:latest
```

## Destroying

```bash
terraform destroy \
  -var="project_id=my-gcp-project" \
  -var="control_plane_url=https://control-plane.example.com"
```

> **Note:** GKE clusters and Artifact Registry repositories with images will be permanently deleted. Ensure you have backups of any data you need before running destroy.
