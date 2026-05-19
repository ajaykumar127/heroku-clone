variable "resource_group_name" {
  description = "Name of the Azure Resource Group to create."
  type        = string
  default     = "platform-runtime-rg"
}

variable "location" {
  description = "Azure region where all resources will be provisioned."
  type        = string
  default     = "East US"
}

variable "cluster_name" {
  description = "Name of the AKS cluster."
  type        = string
  default     = "platform-runtime"
}

variable "control_plane_url" {
  description = "HTTPS URL of the central control-plane API that the runtime agent connects to. Required."
  type        = string

  validation {
    condition     = can(regex("^https://", var.control_plane_url))
    error_message = "control_plane_url must begin with 'https://'."
  }
}

variable "runtime_name" {
  description = "Logical name that identifies this runtime plane within the platform."
  type        = string
  default     = "azure-eastus"
}

variable "kubernetes_version" {
  description = "Kubernetes version to use for the AKS cluster."
  type        = string
  default     = "1.28"
}

variable "node_vm_size" {
  description = "VM size for the default AKS node pool."
  type        = string
  default     = "Standard_D2s_v3"
}

variable "node_min_count" {
  description = "Minimum number of nodes in the default node pool (autoscaler lower bound)."
  type        = number
  default     = 1
}

variable "node_max_count" {
  description = "Maximum number of nodes in the default node pool (autoscaler upper bound)."
  type        = number
  default     = 5
}

variable "node_count" {
  description = "Initial / desired number of nodes in the default node pool."
  type        = number
  default     = 2
}

variable "environment" {
  description = "Deployment environment tag (e.g. production, staging, development)."
  type        = string
  default     = "production"

  validation {
    condition     = contains(["production", "staging", "development"], var.environment)
    error_message = "environment must be one of: production, staging, development."
  }
}
