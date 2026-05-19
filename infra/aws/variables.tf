variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "platform-runtime"
}

variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "control_plane_url" {
  description = "URL of the control plane API (required)"
  type        = string
}

variable "runtime_name" {
  description = "Logical name for this runtime plane"
  type        = string
  default     = "aws-us-east-1"
}

variable "kubernetes_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.28"
}

variable "node_instance_type" {
  description = "EC2 instance type for EKS managed node group"
  type        = string
  default     = "t3.medium"
}

variable "node_min_count" {
  description = "Minimum number of nodes in the managed node group"
  type        = number
  default     = 1
}

variable "node_max_count" {
  description = "Maximum number of nodes in the managed node group"
  type        = number
  default     = 5
}

variable "node_desired_count" {
  description = "Desired number of nodes in the managed node group"
  type        = number
  default     = 2
}

variable "environment" {
  description = "Deployment environment (e.g. production, staging)"
  type        = string
  default     = "production"
}

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

variable "agent_image" {
  description = "Container image for the runtime agent"
  type        = string
  default     = "ghcr.io/platform/runtime-agent:latest"
}
