# AWS Runtime Plane — Terraform

Provisions a production-ready AWS Runtime Plane for the Heroku-clone PaaS platform.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) configured with credentials that have sufficient IAM permissions
- [kubectl](https://kubernetes.io/docs/tasks/tools/) for interacting with the cluster after provisioning

## Quick Start

```bash
terraform init

terraform plan -var="control_plane_url=https://your-control-plane.com"

terraform apply -var="control_plane_url=https://your-control-plane.com"
```

Override other defaults as needed:

```bash
terraform apply \
  -var="control_plane_url=https://your-control-plane.com" \
  -var="cluster_name=my-runtime" \
  -var="region=us-west-2" \
  -var="node_desired_count=3"
```

## What Gets Created

- **VPC** — `/16` with 3 public and 3 private subnets across 3 AZs, an Internet Gateway, and a single NAT Gateway
- **EKS Cluster** — Kubernetes 1.28 with API, audit, and authenticator logging enabled
- **Managed Node Group** — Auto-scaling EC2 nodes (default `t3.medium`, min 1 / max 5 / desired 2)
- **EKS Add-ons** — `vpc-cni`, `coredns`, `kube-proxy`, `aws-ebs-csi-driver`
- **ECR Repository** — `platform-apps` with image scanning on push and a lifecycle policy (last 30 tagged images; untagged purged after 7 days)
- **IAM Roles** — Cluster role, node group role, and an IRSA role for the runtime agent pod
- **OIDC Provider** — Enables IAM Roles for Service Accounts (IRSA) on the cluster
- **Runtime Agent Deployment** — Deployed into the `platform-system` namespace, wired to the control plane via environment variables
- **nginx-ingress-controller** — Deployed via Helm into the `ingress-nginx` namespace, backed by an AWS NLB

## Connecting kubectl

After `terraform apply` completes, run the command printed by the `kubeconfig_command` output:

```bash
aws eks update-kubeconfig --name platform-runtime --region us-east-1
```

Then verify access:

```bash
kubectl get nodes
kubectl get pods -n platform-system
```

## Key Variables

| Variable | Default | Description |
|---|---|---|
| `control_plane_url` | *(required)* | URL of the control plane API |
| `cluster_name` | `platform-runtime` | EKS cluster name |
| `region` | `us-east-1` | AWS region |
| `runtime_name` | `aws-us-east-1` | Logical name for this runtime plane |
| `kubernetes_version` | `1.28` | Kubernetes version |
| `node_instance_type` | `t3.medium` | EC2 instance type for nodes |
| `node_min_count` | `1` | Minimum node count |
| `node_max_count` | `5` | Maximum node count |
| `node_desired_count` | `2` | Desired node count |
| `environment` | `production` | Environment tag value |
| `agent_image` | `ghcr.io/platform/runtime-agent:latest` | Runtime agent container image |

## Destroying

```bash
terraform destroy -var="control_plane_url=https://your-control-plane.com"
```

> **Note:** ECR images and EKS logs are not automatically deleted. Remove them manually if needed to avoid ongoing storage costs.
