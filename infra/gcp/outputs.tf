output "cluster_endpoint" {
  description = "The public IP address of the GKE cluster API server."
  value       = google_container_cluster.platform.endpoint
  sensitive   = true
}

output "cluster_name" {
  description = "The name of the GKE cluster."
  value       = google_container_cluster.platform.name
}

output "registry_url" {
  description = "Base URL of the Artifact Registry Docker repository."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/platform-apps"
}

output "kubeconfig_command" {
  description = "Run this command to configure kubectl for the cluster."
  value       = "gcloud container clusters get-credentials ${google_container_cluster.platform.name} --region ${var.region} --project ${var.project_id}"
}

output "workload_identity_sa_email" {
  description = "Email of the GCP service account used by the runtime agent via Workload Identity."
  value       = google_service_account.runtime_agent.email
}
