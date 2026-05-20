# ---------------------------------------------------------------------------
# User-Assigned Managed Identity for the runtime agent pod
# ---------------------------------------------------------------------------
resource "azurerm_user_assigned_identity" "runtime_agent" {
  name                = "${var.cluster_name}-runtime-agent"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  tags                = local.tags
}

# ---------------------------------------------------------------------------
# Federated Identity Credential — binds the managed identity to the
# Kubernetes service account via OIDC Workload Identity.
# ---------------------------------------------------------------------------
resource "azurerm_federated_identity_credential" "runtime_agent" {
  name                = "${var.cluster_name}-runtime-agent-federated"
  resource_group_name = azurerm_resource_group.main.name
  parent_id           = azurerm_user_assigned_identity.runtime_agent.id

  # The OIDC issuer URL exposed by the AKS cluster.
  issuer = azurerm_kubernetes_cluster.main.oidc_issuer_url

  # Must match the namespace/service-account used in k8s-runtime-agent.tf.
  subject = "system:serviceaccount:platform-system:runtime-agent"

  audience = ["api://AzureADTokenExchange"]
}

# ---------------------------------------------------------------------------
# AcrPush — allows the runtime agent to push built images to ACR.
# ---------------------------------------------------------------------------
resource "azurerm_role_assignment" "agent_acr_push" {
  scope                = azurerm_container_registry.main.id
  role_definition_name = "AcrPush"
  principal_id         = azurerm_user_assigned_identity.runtime_agent.principal_id
}

# ---------------------------------------------------------------------------
# Custom role — minimal permissions for the Platform runtime plane.
# Replaces the over-privileged Contributor role with only the actions the
# runtime agent actually needs: AKS management, ACR access, network reads,
# resource group read, and managed identity reads for OIDC/Workload Identity.
# ---------------------------------------------------------------------------
resource "azurerm_role_definition" "platform_runtime" {
  name        = "PlatformRuntimeRole-${var.resource_group_name}"
  scope       = azurerm_resource_group.main.id
  description = "Minimal permissions for Platform runtime plane — AKS, ACR, network read"

  permissions {
    actions = [
      # AKS
      "Microsoft.ContainerService/managedClusters/read",
      "Microsoft.ContainerService/managedClusters/listClusterUserCredential/action",
      "Microsoft.ContainerService/managedClusters/agentPools/read",
      "Microsoft.ContainerService/managedClusters/agentPools/write",
      # ACR
      "Microsoft.ContainerRegistry/registries/read",
      "Microsoft.ContainerRegistry/registries/pull/read",
      "Microsoft.ContainerRegistry/registries/push/write",
      "Microsoft.ContainerRegistry/registries/listCredentials/action",
      # Network (read-only for AKS node attachment)
      "Microsoft.Network/virtualNetworks/read",
      "Microsoft.Network/virtualNetworks/subnets/read",
      "Microsoft.Network/virtualNetworks/subnets/join/action",
      # Resource group (read only)
      "Microsoft.Resources/subscriptions/resourceGroups/read",
      # Managed identity (for OIDC)
      "Microsoft.ManagedIdentity/userAssignedIdentities/read",
      "Microsoft.ManagedIdentity/userAssignedIdentities/assign/action",
    ]
    not_actions = []
  }

  assignable_scopes = [azurerm_resource_group.main.id]
}

# ---------------------------------------------------------------------------
# Custom role assignment — binds the platform_runtime custom role to the
# runtime agent managed identity on the resource group scope.
# Replaces the previous Contributor assignment to reduce blast radius.
# ---------------------------------------------------------------------------
resource "azurerm_role_assignment" "agent_rg_contributor" {
  scope              = azurerm_resource_group.main.id
  role_definition_id = azurerm_role_definition.platform_runtime.role_definition_resource_id
  principal_id       = azurerm_user_assigned_identity.runtime_agent.principal_id
}
