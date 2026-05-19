# ---------------------------------------------------------------------------
# Log Analytics Workspace (for cluster diagnostics)
# ---------------------------------------------------------------------------
resource "azurerm_log_analytics_workspace" "main" {
  name                = "${var.cluster_name}-logs"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = local.tags
}

# ---------------------------------------------------------------------------
# AKS Cluster
#
# NOTE: private_cluster_enabled is set to false here for development
# simplicity. For production workloads you should set this to true and
# provision a private DNS zone + VPN/ExpressRoute/bastion for API access.
# ---------------------------------------------------------------------------
resource "azurerm_kubernetes_cluster" "main" {
  name                = var.cluster_name
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  dns_prefix          = var.cluster_name
  kubernetes_version  = var.kubernetes_version

  # Auto-upgrade — keeps nodes on the latest patch within the minor version.
  automatic_channel_upgrade = "stable"

  # Private cluster disabled for simplicity; enable for production.
  private_cluster_enabled = false

  # Workload Identity & OIDC issuer
  workload_identity_enabled = true
  oidc_issuer_enabled       = true

  # System node pool
  default_node_pool {
    name                = "system"
    vm_size             = var.node_vm_size
    node_count          = var.node_count
    min_count           = var.node_min_count
    max_count           = var.node_max_count
    enable_auto_scaling = true
    vnet_subnet_id      = azurerm_subnet.aks.id
    os_disk_size_gb     = 50

    # Ensures system pods land on this pool
    only_critical_addons_enabled = false

    node_labels = {
      "platform/node-pool" = "system"
    }
  }

  # Azure CNI networking
  network_profile {
    network_plugin    = "azure"
    load_balancer_sku = "standard"
    service_cidr      = "10.0.0.0/16"
    dns_service_ip    = "10.0.0.10"
  }

  # Managed identity for the control plane
  identity {
    type = "SystemAssigned"
  }

  # OMS agent for container insights
  oms_agent {
    log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  }

  tags = local.tags
}

# ---------------------------------------------------------------------------
# Diagnostic settings — ship kube-apiserver & scheduler logs to Log Analytics
# ---------------------------------------------------------------------------
resource "azurerm_monitor_diagnostic_setting" "aks" {
  name                       = "${var.cluster_name}-diagnostics"
  target_resource_id         = azurerm_kubernetes_cluster.main.id
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id

  enabled_log {
    category = "kube-apiserver"
  }

  enabled_log {
    category = "kube-scheduler"
  }

  enabled_log {
    category = "kube-controller-manager"
  }

  enabled_log {
    category = "kube-audit"
  }

  enabled_log {
    category = "cluster-autoscaler"
  }

  metric {
    category = "AllMetrics"
    enabled  = true
  }
}
