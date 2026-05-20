# ---------------------------------------------------------------------------
# Namespace — platform-system houses cluster-level platform components.
# ---------------------------------------------------------------------------
resource "kubernetes_namespace" "platform_system" {
  metadata {
    name = "platform-system"

    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
      "platform/component"           = "runtime-agent"
    }
  }

  depends_on = [azurerm_kubernetes_cluster.main]
}

resource "kubernetes_namespace" "platform_apps" {
  metadata {
    name = "platform-apps"

    labels = {
      "app.kubernetes.io/managed-by" = "terraform"
      "platform/component"           = "workloads"
    }
  }

  depends_on = [azurerm_kubernetes_cluster.main]
}

# ---------------------------------------------------------------------------
# Service Account — annotated with Workload Identity client ID so the pod
# can exchange its projected service-account token for an Azure AD token.
# ---------------------------------------------------------------------------
resource "kubernetes_service_account" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name

    annotations = {
      "azure.workload.identity/client-id" = azurerm_user_assigned_identity.runtime_agent.client_id
    }

    labels = {
      "azure.workload.identity/use"  = "true"
      "app.kubernetes.io/name"       = "runtime-agent"
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }
}

# ---------------------------------------------------------------------------
# Deployment — runtime agent
# ---------------------------------------------------------------------------
resource "kubernetes_deployment" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name

    labels = {
      "app"                          = "runtime-agent"
      "app.kubernetes.io/name"       = "runtime-agent"
      "app.kubernetes.io/managed-by" = "terraform"
    }
  }

  spec {
    replicas = 2

    selector {
      match_labels = {
        "app" = "runtime-agent"
      }
    }

    template {
      metadata {
        labels = {
          "app"                         = "runtime-agent"
          "azure.workload.identity/use" = "true"
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
          # Pin to a specific version tag — never use :latest in production.
          image = "${azurerm_container_registry.main.login_server}/platform/runtime-agent:${var.agent_image_tag}"

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
            value = "azure"
          }

          env {
            name  = "REGION"
            value = var.location
          }

          env {
            name  = "CONTROL_PLANE_URL"
            value = var.control_plane_url
          }

          env {
            name  = "REGISTRY_URL"
            value = azurerm_container_registry.main.login_server
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

          port {
            name           = "http"
            container_port = 8080
            protocol       = "TCP"
          }
        }

        volume {
          name = "tmp"
          empty_dir {}
        }

        topology_spread_constraint {
          max_skew           = 1
          topology_key       = "kubernetes.io/hostname"
          when_unsatisfiable = "DoNotSchedule"

          label_selector {
            match_labels = {
              "app" = "runtime-agent"
            }
          }
        }
      }
    }
  }

  depends_on = [
    azurerm_kubernetes_cluster.main,
    azurerm_role_assignment.agent_acr_push,
    azurerm_role_assignment.aks_acr_pull,
  ]
}

# ---------------------------------------------------------------------------
# nginx Ingress Controller (Azure-flavoured)
#
# Uses the official ingress-nginx Helm chart.  The LoadBalancer service is
# annotated to provision an Azure Standard Load Balancer with a public IP.
# For internal-only ingress, swap the annotation value to "true".
# ---------------------------------------------------------------------------
resource "helm_release" "nginx_ingress" {
  name             = "ingress-nginx"
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  version          = "4.8.3"
  namespace        = "ingress-nginx"
  create_namespace = true
  atomic           = true
  timeout          = 300

  set {
    name  = "controller.replicaCount"
    value = "2"
  }

  # Azure-specific: provision a public Standard Load Balancer.
  set {
    name  = "controller.service.annotations.service\\.beta\\.kubernetes\\.io/azure-load-balancer-health-probe-request-path"
    value = "/healthz"
  }

  set {
    name  = "controller.service.annotations.service\\.beta\\.kubernetes\\.io/azure-load-balancer-internal"
    value = "false"
  }

  # Enable Prometheus metrics scraping.
  set {
    name  = "controller.metrics.enabled"
    value = "true"
  }

  # Node affinity: prefer system node pool for ingress pods.
  set {
    name  = "controller.nodeSelector.kubernetes\\.io/os"
    value = "linux"
  }

  depends_on = [kubernetes_namespace.platform_system]
}
