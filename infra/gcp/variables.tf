variable "project_id" {
  description = "The GCP project ID in which all resources will be created."
  type        = string
}

variable "region" {
  description = "The GCP region for all resources."
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "The name of the GKE cluster."
  type        = string
  default     = "platform-runtime"
}

variable "control_plane_url" {
  description = "URL of the PaaS control plane API that the runtime agent connects to."
  type        = string
}

variable "runtime_name" {
  description = "Unique identifier for this runtime plane (reported to the control plane)."
  type        = string
  default     = "gcp-us-central1"
}

variable "kubernetes_version" {
  description = "Minimum Kubernetes master version to use (prefix matched against available versions in the release channel)."
  type        = string
  default     = "1.28"
}

variable "node_machine_type" {
  description = "GCE machine type for GKE worker nodes."
  type        = string
  default     = "e2-standard-2"
}

variable "node_min_count" {
  description = "Minimum number of nodes in the node pool (autoscaling lower bound)."
  type        = number
  default     = 1
}

variable "node_max_count" {
  description = "Maximum number of nodes in the node pool (autoscaling upper bound)."
  type        = number
  default     = 5
}

variable "node_initial_count" {
  description = "Initial number of nodes in the node pool at cluster creation time."
  type        = number
  default     = 2
}

variable "environment" {
  description = "Deployment environment label (e.g. production, staging)."
  type        = string
  default     = "production"
}

variable "master_authorized_cidr" {
  description = "CIDR range allowed to reach the GKE API server. Defaults to unrestricted; tighten in production."
  type        = string
  default     = "0.0.0.0/0"
}

variable "agent_image_tag" {
  description = "Runtime agent container image tag. Pin to a specific version tag — never use :latest in production."
  type        = string
  default     = "v0.1.0"
}
