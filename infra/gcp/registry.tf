# ---------------------------------------------------------------------------
# Artifact Registry — Docker repository for platform app images
# ---------------------------------------------------------------------------

resource "google_artifact_registry_repository" "platform_apps" {
  location      = var.region
  repository_id = "platform-apps"
  format        = "DOCKER"
  description   = "Docker images for platform-hosted applications (${var.environment})"

  labels = {
    environment = var.environment
    managed-by  = "terraform"
  }
}

# Grant GKE nodes read access so they can pull images at runtime.
resource "google_artifact_registry_repository_iam_member" "node_reader" {
  location   = google_artifact_registry_repository.platform_apps.location
  repository = google_artifact_registry_repository.platform_apps.name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.gke_node.email}"
}
