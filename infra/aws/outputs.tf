output "cluster_endpoint" {
  description = "EKS API server URL"
  value       = aws_eks_cluster.main.endpoint
}

output "cluster_name" {
  description = "Name of the EKS cluster"
  value       = aws_eks_cluster.main.name
}

output "ecr_repository_url" {
  description = "URL of the ECR repository for platform applications"
  value       = aws_ecr_repository.platform_apps.repository_url
}

output "kubeconfig_command" {
  description = "AWS CLI command to update local kubeconfig for kubectl access"
  value       = "aws eks update-kubeconfig --name ${aws_eks_cluster.main.name} --region ${var.region}"
}

output "runtime_agent_iam_role_arn" {
  description = "ARN of the IRSA IAM role for the runtime agent"
  value       = aws_iam_role.runtime_agent.arn
}
