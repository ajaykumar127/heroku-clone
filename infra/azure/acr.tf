# ---------------------------------------------------------------------------
# Random suffix — keeps the registry name globally unique
# ---------------------------------------------------------------------------
resource "random_string" "acr_suffix" {
  length  = 6
  upper   = false
  special = false
  numeric = true
}

# ---------------------------------------------------------------------------
# Azure Container Registry
#
# NOTE: Geo-replication is intentionally not configured here because each
# replica incurs the cost of a full registry instance. To enable it, add an
# azurerm_container_registry_geo_replication resource (or a replication block
# inside the registry) pointing at your secondary regions.
# ---------------------------------------------------------------------------
resource "azurerm_container_registry" "main" {
  name                = "platformregistry${random_string.acr_suffix.result}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "Standard"

  # Disable admin credentials — access is controlled via Azure RBAC / managed identities.
  admin_enabled = false

  tags = local.tags
}

# ---------------------------------------------------------------------------
# AcrPull — grants the AKS kubelet identity permission to pull images.
# This is required for Kubernetes to pull application images without
# explicit imagePullSecrets.
# ---------------------------------------------------------------------------
resource "azurerm_role_assignment" "aks_acr_pull" {
  scope                = azurerm_container_registry.main.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_kubernetes_cluster.main.kubelet_identity[0].object_id
}
