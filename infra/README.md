# Infrastructure Provisioning

This directory contains Terraform configurations for provisioning Runtime Plane infrastructure on each supported cloud provider. Each sub-directory is self-contained: you apply one to get a fully configured Kubernetes cluster, container registry, and supporting networking that can be registered as a Runtime Plane with the Control Plane.

## Directory Layout

```
infra/
├── aws/     # EKS cluster + ECR registry + VPC
├── gcp/     # GKE cluster + Artifact Registry + VPC
└── azure/   # AKS cluster + ACR registry + VNet
```

## Common Workflow

The pattern is the same for every cloud:

```bash
cd infra/<cloud>
terraform init
terraform apply
```

After `terraform apply` completes, the Runtime Agent is deployed into the cluster as a Kubernetes Deployment. It reads its configuration (Control Plane URL, cloud credentials) from a Kubernetes Secret created by Terraform, and **automatically registers itself** with the Control Plane on first startup. You can verify registration with:

```bash
platform runtime list
```

## Cloud-Specific Variables

### AWS (`infra/aws/`)

| Variable              | Description                                      | Example                          |
|-----------------------|--------------------------------------------------|----------------------------------|
| `region`              | AWS region to deploy into                        | `us-east-1`                      |
| `cluster_name`        | Name for the EKS cluster                         | `platform-runtime-us-east-1`     |
| `node_instance_type`  | EC2 instance type for worker nodes               | `m5.xlarge`                      |
| `node_count`          | Number of worker nodes                           | `3`                              |
| `control_plane_url`   | URL of the Control Plane API server              | `https://api.platform.example.com` |
| `control_plane_token` | Bootstrap token for agent registration           | (from Control Plane admin)       |
| `ecr_repo_prefix`     | Prefix for ECR repository names                  | `platform`                       |

### GCP (`infra/gcp/`)

| Variable              | Description                                      | Example                          |
|-----------------------|--------------------------------------------------|----------------------------------|
| `project_id`          | GCP project ID                                   | `my-platform-project`            |
| `region`              | GCP region to deploy into                        | `us-central1`                    |
| `cluster_name`        | Name for the GKE cluster                         | `platform-runtime-us-central1`   |
| `node_machine_type`   | GCE machine type for worker nodes                | `n2-standard-4`                  |
| `node_count`          | Number of worker nodes                           | `3`                              |
| `control_plane_url`   | URL of the Control Plane API server              | `https://api.platform.example.com` |
| `control_plane_token` | Bootstrap token for agent registration           | (from Control Plane admin)       |
| `gar_location`        | Artifact Registry location                       | `us-central1`                    |

### Azure (`infra/azure/`)

| Variable              | Description                                      | Example                          |
|-----------------------|--------------------------------------------------|----------------------------------|
| `location`            | Azure region to deploy into                      | `eastus`                         |
| `resource_group`      | Azure resource group name                        | `platform-runtime-eastus`        |
| `cluster_name`        | Name for the AKS cluster                         | `platform-runtime-eastus`        |
| `node_vm_size`        | VM size for worker nodes                         | `Standard_D4s_v3`                |
| `node_count`          | Number of worker nodes                           | `3`                              |
| `control_plane_url`   | URL of the Control Plane API server              | `https://api.platform.example.com` |
| `control_plane_token` | Bootstrap token for agent registration           | (from Control Plane admin)       |
| `acr_name`            | Name for the Azure Container Registry            | `platformruntimeeastus`          |

## How Runtime Registration Works

The Runtime Agent container image is pre-built and published to a registry accessible from all clouds. When deployed, it performs the following on startup:

1. Reads `CONTROL_PLANE_URL`, `CONTROL_PLANE_TOKEN`, `RUNTIME_NAME`, `RUNTIME_CLOUD`, and `RUNTIME_REGION` from environment variables (injected from the Kubernetes Secret created by Terraform).
2. Calls `POST /internal/runtime/register` with its name, cloud, region, and agent URL.
3. Begins polling `GET /internal/jobs/next` every 5 seconds for build and deploy jobs.
4. Sends a heartbeat to `PATCH /internal/runtime/<id>/heartbeat` every 30 seconds so the Control Plane can track the `last_seen` timestamp and mark the runtime `active`.

For **manual registration** (e.g. a local or custom setup without Terraform):

```bash
platform runtime register \
  --name local-dev \
  --cloud local \
  --region localhost \
  --agent-url http://localhost:9090
```

To remove a runtime, stop or delete the Runtime Agent pod. The Control Plane will mark it `inactive` after two missed heartbeat intervals. Full deregistration support is planned for a future release:

```bash
platform runtime deregister local-dev
# Not yet implemented. Remove the runtime agent pod from your cluster to deregister.
```
