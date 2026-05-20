# ---------------------------------------------------------------------------
# Kubernetes namespace
# ---------------------------------------------------------------------------

resource "kubernetes_namespace" "platform_system" {
  metadata {
    name = "platform-system"
    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
      environment                    = var.environment
    }
  }

  depends_on = [google_container_node_pool.platform_nodes]
}

resource "kubernetes_namespace" "platform_apps" {
  metadata {
    name = "platform-apps"
    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
      environment                    = var.environment
    }
  }

  depends_on = [google_container_node_pool.platform_nodes]
}

# ---------------------------------------------------------------------------
# Kubernetes service account with Workload Identity annotation
# ---------------------------------------------------------------------------

resource "kubernetes_service_account" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name

    annotations = {
      # Bind this K8s SA to the GCP SA via Workload Identity.
      "iam.gke.io/gcp-service-account" = google_service_account.runtime_agent.email
    }

    labels = {
      "app.kubernetes.io/name"       = "runtime-agent"
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }
}

# ---------------------------------------------------------------------------
# Runtime agent Deployment
# ---------------------------------------------------------------------------

locals {
  registry_url = "${var.region}-docker.pkg.dev/${var.project_id}/platform-apps"
}

resource "kubernetes_deployment" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name

    labels = {
      "app.kubernetes.io/name"       = "runtime-agent"
      "app.kubernetes.io/component"  = "runtime-agent"
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app.kubernetes.io/name"      = "runtime-agent"
        "app.kubernetes.io/component" = "runtime-agent"
      }
    }

    template {
      metadata {
        labels = {
          "app.kubernetes.io/name"      = "runtime-agent"
          "app.kubernetes.io/component" = "runtime-agent"
        }
      }

      spec {
        service_account_name            = kubernetes_service_account.runtime_agent.metadata[0].name
        automount_service_account_token = false

        security_context {
          run_as_non_root = true
          run_as_user     = 65534
          run_as_group    = 65534
          fs_group        = 65534
        }

        container {
          name  = "runtime-agent"
          image = "${local.registry_url}/runtime-agent:${var.agent_image_tag}"

          image_pull_policy = "Always"

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_non_root            = true
            capabilities {
              drop = ["ALL"]
            }
          }

          env {
            name  = "RUNTIME_NAME"
            value = var.runtime_name
          }

          env {
            name  = "CLOUD"
            value = "gcp"
          }

          env {
            name  = "REGION"
            value = var.region
          }

          env {
            name  = "CONTROL_PLANE_URL"
            value = var.control_plane_url
          }

          env {
            name  = "REGISTRY_URL"
            value = local.registry_url
          }

          env {
            name  = "APPS_NAMESPACE"
            value = "platform-apps"
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "128Mi"
            }
            limits = {
              cpu    = "500m"
              memory = "512Mi"
            }
          }

          volume_mount {
            name       = "tmp"
            mount_path = "/tmp"
          }

          liveness_probe {
            http_get {
              path = "/healthz"
              port = 8080
            }
            initial_delay_seconds = 15
            period_seconds        = 20
          }

          readiness_probe {
            http_get {
              path = "/readyz"
              port = 8080
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }
        }

        volume {
          name = "tmp"
          empty_dir {}
        }
      }
    }
  }

  depends_on = [
    google_service_account_iam_member.workload_identity_binding,
    google_container_node_pool.platform_nodes,
  ]
}

# ---------------------------------------------------------------------------
# nginx-ingress-controller via Helm
# ---------------------------------------------------------------------------

resource "helm_release" "nginx_ingress" {
  name             = "ingress-nginx"
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  version          = "4.9.1"
  namespace        = "ingress-nginx"
  create_namespace = true

  set {
    name  = "controller.service.type"
    value = "LoadBalancer"
  }

  set {
    name  = "controller.replicaCount"
    value = "2"
  }

  set {
    name  = "controller.resources.requests.cpu"
    value = "100m"
  }

  set {
    name  = "controller.resources.requests.memory"
    value = "90Mi"
  }

  set {
    name  = "controller.metrics.enabled"
    value = "true"
  }

  depends_on = [google_container_node_pool.platform_nodes]
}
