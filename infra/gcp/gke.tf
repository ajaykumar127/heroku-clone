# ---------------------------------------------------------------------------
# GKE — Standard (non-Autopilot) private cluster
# ---------------------------------------------------------------------------

resource "google_container_cluster" "platform" {
  name     = var.cluster_name
  location = var.region

  # We manage the node pool separately; delete the default one.
  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.platform.id
  subnetwork = google_compute_subnetwork.platform.id

  # -------------------------------------------------------------------------
  # Networking
  # -------------------------------------------------------------------------
  networking_mode = "VPC_NATIVE"

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Private cluster — nodes get no public IPs.
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false # Public endpoint for kubectl access
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = var.master_authorized_cidr
      display_name = "all (tighten in production)"
    }
  }

  # -------------------------------------------------------------------------
  # Kubernetes version / release channel
  # -------------------------------------------------------------------------
  release_channel {
    channel = "REGULAR"
  }

  min_master_version = var.kubernetes_version

  # -------------------------------------------------------------------------
  # Workload Identity
  # -------------------------------------------------------------------------
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # -------------------------------------------------------------------------
  # Network policy (Calico)
  # -------------------------------------------------------------------------
  network_policy {
    enabled  = true
    provider = "CALICO"
  }

  # Required when network_policy provider is CALICO
  addons_config {
    network_policy_config {
      disabled = false
    }
    http_load_balancing {
      disabled = false
    }
    horizontal_pod_autoscaling {
      disabled = false
    }
  }

  # -------------------------------------------------------------------------
  # Cloud Operations (logging + monitoring)
  # -------------------------------------------------------------------------
  logging_service    = "logging.googleapis.com/kubernetes"
  monitoring_service = "monitoring.googleapis.com/kubernetes"

  # -------------------------------------------------------------------------
  # Misc hardening
  # -------------------------------------------------------------------------
  enable_shielded_nodes = true

  lifecycle {
    ignore_changes = [initial_node_count]
  }
}

# ---------------------------------------------------------------------------
# Node pool
# ---------------------------------------------------------------------------

resource "google_container_node_pool" "platform_nodes" {
  name     = "${var.cluster_name}-nodes"
  location = var.region
  cluster  = google_container_cluster.platform.name

  initial_node_count = var.node_initial_count

  autoscaling {
    min_node_count = var.node_min_count
    max_node_count = var.node_max_count
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    machine_type    = var.node_machine_type
    service_account = google_service_account.gke_node.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]

    # Workload Identity on the node pool
    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }

    labels = {
      environment = var.environment
      managed-by  = "terraform"
    }

    tags = ["gke-node", var.cluster_name]
  }

  lifecycle {
    ignore_changes = [initial_node_count]
  }
}

# ---------------------------------------------------------------------------
# Kubernetes + Helm providers — configured after the cluster is ready
# ---------------------------------------------------------------------------

data "google_client_config" "default" {}

provider "kubernetes" {
  host                   = "https://${google_container_cluster.platform.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(google_container_cluster.platform.master_auth[0].cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = "https://${google_container_cluster.platform.endpoint}"
    token                  = data.google_client_config.default.access_token
    cluster_ca_certificate = base64decode(google_container_cluster.platform.master_auth[0].cluster_ca_certificate)
  }
}
