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
# Contributor on the resource group — allows the runtime agent to manage
# platform resources (deploy, scale, etc.).
#
# NOTE: For production, scope this down to only the specific resource types
# the agent needs (e.g. a dedicated "platform-apps" resource group) and
# prefer purpose-built roles over Contributor to reduce the blast radius.
# ---------------------------------------------------------------------------
resource "azurerm_role_assignment" "agent_rg_contributor" {
  scope                = azurerm_resource_group.main.id
  role_definition_name = "Contributor"
  principal_id         = azurerm_user_assigned_identity.runtime_agent.principal_id
}
