output "cluster_name" {
  description = "Name of the provisioned AKS cluster."
  value       = azurerm_kubernetes_cluster.main.name
}

output "resource_group_name" {
  description = "Name of the Azure resource group that contains all platform resources."
  value       = azurerm_resource_group.main.name
}

output "acr_login_server" {
  description = "Fully-qualified login server hostname for the Azure Container Registry."
  value       = azurerm_container_registry.main.login_server
}

output "kubeconfig_command" {
  description = "az CLI command to merge AKS credentials into your local kubeconfig."
  value       = "az aks get-credentials --resource-group ${azurerm_resource_group.main.name} --name ${azurerm_kubernetes_cluster.main.name}"
}

output "workload_identity_client_id" {
  description = "Azure AD client ID of the runtime-agent user-assigned managed identity. Used in the service account annotation."
  value       = azurerm_user_assigned_identity.runtime_agent.client_id
}

output "oidc_issuer_url" {
  description = "OIDC issuer URL for the AKS cluster. Used when configuring additional federated identity credentials."
  value       = azurerm_kubernetes_cluster.main.oidc_issuer_url
}

output "log_analytics_workspace_id" {
  description = "Resource ID of the Log Analytics workspace used for cluster diagnostics."
  value       = azurerm_log_analytics_workspace.main.id
}
