# ---------------------------------------------------------------------------
# VPC — custom subnetwork mode so GKE secondary ranges can be declared
# ---------------------------------------------------------------------------

resource "google_compute_network" "platform" {
  name                    = "platform-vpc"
  auto_create_subnetworks = false
  description             = "Platform runtime VPC (${var.environment})"
}

resource "google_compute_subnetwork" "platform" {
  name          = "platform-subnet-${var.region}"
  ip_cidr_range = "10.0.0.0/16"
  region        = var.region
  network       = google_compute_network.platform.id
  description   = "Primary subnet for GKE nodes"

  # Required for Workload Identity / Private Google Access
  private_ip_google_access = true

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.100.0.0/14"
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.104.0.0/20"
  }
}

# ---------------------------------------------------------------------------
# Cloud Router + Cloud NAT — allow private nodes to reach the internet
# ---------------------------------------------------------------------------

resource "google_compute_router" "platform" {
  name    = "platform-router"
  region  = var.region
  network = google_compute_network.platform.id
}

resource "google_compute_router_nat" "platform" {
  name                               = "platform-nat"
  router                             = google_compute_router.platform.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}
