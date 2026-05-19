# -------------------------------------------------------
# Namespace
# -------------------------------------------------------
resource "kubernetes_namespace" "platform_system" {
  metadata {
    name = "platform-system"
    labels = {
      name        = "platform-system"
      environment = var.environment
      managed_by  = "terraform"
    }
  }

  depends_on = [aws_eks_node_group.main]
}

# -------------------------------------------------------
# Service Account (annotated with IRSA role ARN)
# -------------------------------------------------------
resource "kubernetes_service_account" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name
    annotations = {
      "eks.amazonaws.com/role-arn" = aws_iam_role.runtime_agent.arn
    }
    labels = {
      app        = "runtime-agent"
      managed_by = "terraform"
    }
  }
}

# -------------------------------------------------------
# Runtime Agent Deployment
# -------------------------------------------------------
resource "kubernetes_deployment" "runtime_agent" {
  metadata {
    name      = "runtime-agent"
    namespace = kubernetes_namespace.platform_system.metadata[0].name
    labels = {
      app        = "runtime-agent"
      managed_by = "terraform"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "runtime-agent"
      }
    }

    template {
      metadata {
        labels = {
          app = "runtime-agent"
        }
      }

      spec {
        service_account_name            = kubernetes_service_account.runtime_agent.metadata[0].name
        automount_service_account_token = true

        container {
          name  = "runtime-agent"
          image = var.agent_image

          env {
            name  = "RUNTIME_NAME"
            value = var.runtime_name
          }

          env {
            name  = "CLOUD"
            value = "aws"
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
            value = aws_ecr_repository.platform_apps.repository_url
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
              memory = "256Mi"
            }
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

        restart_policy = "Always"
      }
    }
  }

  depends_on = [
    aws_eks_addon.coredns,
    aws_eks_addon.vpc_cni,
    aws_eks_addon.kube_proxy,
    aws_iam_role_policy_attachment.runtime_agent,
  ]
}

# -------------------------------------------------------
# nginx-ingress-controller via Helm
# -------------------------------------------------------
resource "helm_release" "nginx_ingress" {
  name             = "ingress-nginx"
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  version          = "4.8.3"
  namespace        = "ingress-nginx"
  create_namespace = true

  set {
    name  = "controller.service.type"
    value = "LoadBalancer"
  }

  set {
    name  = "controller.service.annotations.service\\.beta\\.kubernetes\\.io/aws-load-balancer-type"
    value = "nlb"
  }

  set {
    name  = "controller.metrics.enabled"
    value = "true"
  }

  set {
    name  = "controller.replicaCount"
    value = "2"
  }

  depends_on = [
    aws_eks_node_group.main,
    aws_eks_addon.coredns,
    aws_eks_addon.vpc_cni,
  ]
}
