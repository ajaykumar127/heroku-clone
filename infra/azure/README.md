# Azure Runtime Plane — Terraform

Provisions a complete Azure runtime plane for the platform PaaS. All
resources are created inside a single Azure Resource Group and tagged
consistently for cost tracking and governance.

## What gets created

| Resource | Notes |
|---|---|
| Resource Group | Container for all resources below |
| Virtual Network | 10.0.0.0/8 address space |
| AKS Subnet | 10.240.0.0/16 — node & pod IPs (Azure CNI) |
| AKS Cluster | Autoscaling system node pool, OIDC issuer, Workload Identity |
| Log Analytics Workspace | Cluster diagnostic logs & Container Insights |
| Azure Container Registry | Standard SKU, RBAC-only access |
| User-Assigned Managed Identity | For the runtime-agent pod (Workload Identity) |
| Federated Identity Credential | Binds identity to the `runtime-agent` service account |
| Role Assignments | AcrPull (kubelet), AcrPush + Contributor (runtime-agent) |
| Kubernetes Namespaces | `platform-system`, `platform-apps` |
| Runtime Agent Deployment | 2-replica deployment with health probes |
| nginx Ingress Controller | Azure Load Balancer-backed, via Helm |

## Prerequisites

| Tool | Minimum version |
|---|---|
| [Terraform](https://developer.hashicorp.com/terraform/install) | 1.5 |
| [Azure CLI (`az`)](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) | 2.55 |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | 1.28 |
| [Helm](https://helm.sh/docs/intro/install/) | 3.13 |

You must be authenticated to Azure before running Terraform:

```bash
az login
az account set --subscription "<your-subscription-id>"
```

## Quick start

```bash
cd infra/azure

# Initialise providers and modules
terraform init

# Preview the plan — control_plane_url is the only required variable
terraform plan \
  -var="control_plane_url=https://control.example.com"

# Apply
terraform apply \
  -var="control_plane_url=https://control.example.com"
```

To customise the deployment, override variables on the command line or in a
`.tfvars` file:

```hcl
# terraform.tfvars
resource_group_name = "mycompany-runtime-rg"
location            = "West Europe"
cluster_name        = "mycompany-runtime"
control_plane_url   = "https://control.mycompany.com"
runtime_name        = "azure-westeurope"
kubernetes_version  = "1.28"
node_vm_size        = "Standard_D4s_v3"
node_min_count      = 2
node_max_count      = 10
node_count          = 3
environment         = "production"
```

```bash
terraform apply -var-file="terraform.tfvars"
```

## Connecting kubectl

After a successful `apply`, run the command printed in the
`kubeconfig_command` output:

```bash
$(terraform output -raw kubeconfig_command)
```

Verify the connection:

```bash
kubectl get nodes
kubectl get pods -n platform-system
```

## Runtime agent image

The runtime agent deployment references:

```
<acr_login_server>/platform/runtime-agent:latest
```

Build and push the agent image before (or shortly after) running `apply`:

```bash
ACR=$(terraform output -raw acr_login_server)
az acr login --name "${ACR%%.*}"

docker build -t "${ACR}/platform/runtime-agent:latest" ../../agent/
docker push "${ACR}/platform/runtime-agent:latest"
```

## Workload Identity

The runtime agent pod uses Azure Workload Identity to obtain Azure AD tokens
without storing any credentials. The client ID is available as a Terraform
output:

```bash
terraform output workload_identity_client_id
```

The Kubernetes service account is annotated automatically. No additional
configuration is required.

## Security notes

- **Private cluster**: `private_cluster_enabled` is set to `false` for
  development convenience. Set it to `true` for production and configure
  private DNS + network connectivity (VPN / ExpressRoute / bastion host).
- **Contributor role**: The runtime agent is granted Contributor on the
  resource group. For production, scope this down to a dedicated
  `platform-apps` resource group or craft a custom role.
- **Geo-replication**: ACR geo-replication is not configured (cost). To add
  replicas, use `azurerm_container_registry_geo_replication` or add a
  `georeplications` block inside the registry resource.
- **Admin credentials**: ACR admin access is disabled. Use managed identities
  or service principals with `AcrPull`/`AcrPush` roles.

## Destroying

```bash
terraform destroy \
  -var="control_plane_url=https://control.example.com"
```

> **Warning**: This will permanently delete the AKS cluster, ACR (and all
> images), VNet, and all other resources in the resource group. Ensure you
> have backups of any state you need to preserve.
