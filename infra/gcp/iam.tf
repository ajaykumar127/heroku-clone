# ---------------------------------------------------------------------------
# GKE node service account — minimal permissions for worker nodes
# ---------------------------------------------------------------------------

resource "google_service_account" "gke_node" {
  account_id   = "platform-gke-node"
  display_name = "Platform GKE Node Service Account"
  description  = "Minimal SA used by GKE worker nodes"
}

# Nodes must be able to write logs and metrics.
resource "google_project_iam_member" "node_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.gke_node.email}"
}

resource "google_project_iam_member" "node_metric_writer" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.gke_node.email}"
}

resource "google_project_iam_member" "node_monitoring_viewer" {
  project = var.project_id
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.gke_node.email}"
}

# ---------------------------------------------------------------------------
# Runtime agent GCP service account
# ---------------------------------------------------------------------------

resource "google_service_account" "runtime_agent" {
  account_id   = "platform-runtime-agent"
  display_name = "Platform Runtime Agent Service Account"
  description  = "GCP SA used by the runtime-agent pod via Workload Identity"
}

# Allow the runtime agent to push images to Artifact Registry.
resource "google_project_iam_member" "agent_registry_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.runtime_agent.email}"
}

# Allow the runtime agent to interact with GKE (deploy/manage workloads).
resource "google_project_iam_member" "agent_container_developer" {
  project = var.project_id
  role    = "roles/container.developer"
  member  = "serviceAccount:${google_service_account.runtime_agent.email}"
}

# ---------------------------------------------------------------------------
# Workload Identity binding
# Maps the Kubernetes SA  →  GCP SA so no static credentials are needed.
# ---------------------------------------------------------------------------

resource "google_service_account_iam_member" "workload_identity_binding" {
  service_account_id = google_service_account.runtime_agent.name
  role               = "roles/iam.workloadIdentityUser"

  # The principal is the K8s SA projected through the Workload Identity pool.
  member = "serviceAccount:${var.project_id}.svc.id.goog[platform-system/runtime-agent]"
}
