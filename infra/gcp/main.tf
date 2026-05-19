# ---------------------------------------------------------------------------
# GCP Runtime Plane — entry point
#
# This module provisions a complete GCP runtime plane for the PaaS platform:
#   • VPC with secondary ranges for pods and services   (vpc.tf)
#   • Private GKE Standard cluster + autoscaling nodes  (gke.tf)
#   • Artifact Registry Docker repository               (registry.tf)
#   • IAM service accounts + Workload Identity bindings (iam.tf)
#   • Runtime agent Deployment + nginx ingress via Helm (k8s-runtime-agent.tf)
#
# All resources are tagged/labeled with var.environment for easy cost
# attribution and lifecycle management.
# ---------------------------------------------------------------------------

locals {
  common_labels = {
    environment   = var.environment
    managed-by    = "terraform"
    runtime-plane = var.runtime_name
  }
}
