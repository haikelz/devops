output "cluster_name" {
  description = "GKE cluster name."
  value       = google_container_cluster.primary.name
}

output "cluster_region" {
  description = "GKE cluster region."
  value       = google_container_cluster.primary.location
}

output "cluster_endpoint" {
  description = "Public GKE control-plane endpoint."
  value       = google_container_cluster.primary.endpoint
  sensitive   = true
}

output "get_credentials_command" {
  description = "Command to configure kubectl after Terraform applies successfully."
  value       = "gcloud container clusters get-credentials ${google_container_cluster.primary.name} --region ${google_container_cluster.primary.location} --project ${var.project_id}"
}
