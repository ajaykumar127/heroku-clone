# ---------------------------------------------------------------------------
# Virtual Network
# ---------------------------------------------------------------------------
resource "azurerm_virtual_network" "main" {
  name                = "${var.cluster_name}-vnet"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  address_space       = ["10.0.0.0/8"]
  tags                = local.tags
}

# ---------------------------------------------------------------------------
# AKS Subnet
# 10.240.0.0/16 — node & pod IPs (Azure CNI)
# ---------------------------------------------------------------------------
resource "azurerm_subnet" "aks" {
  name                 = "${var.cluster_name}-aks-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.240.0.0/16"]
}

# ---------------------------------------------------------------------------
# Networking notes
#
# Service CIDR  : 10.0.0.0/16  — set on the AKS cluster (aks.tf)
# DNS service IP: 10.0.0.10    — must be inside the service CIDR
#
# The service CIDR (10.0.0.0/16) and node/pod subnet (10.240.0.0/16) do NOT
# overlap. The overall VNet address space (10.0.0.0/8) covers both ranges.
# ---------------------------------------------------------------------------
