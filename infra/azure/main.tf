# ---------------------------------------------------------------------------
# Resource Group
# ---------------------------------------------------------------------------
resource "azurerm_resource_group" "main" {
  name     = var.resource_group_name
  location = var.location
  tags     = local.tags
}

# ---------------------------------------------------------------------------
# Common locals
# ---------------------------------------------------------------------------
locals {
  tags = {
    environment = var.environment
    project     = "platform"
    managed_by  = "terraform"
    runtime     = var.runtime_name
  }
}
