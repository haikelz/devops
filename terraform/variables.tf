variable "project_id" {
  description = "GCP project ID that will contain the GKE cluster."
  type        = string
}

variable "region" {
  description = "GCP region for the regional GKE cluster."
  type        = string
}

variable "cluster_name" {
  description = "Name for the GKE cluster and its network resources."
  type        = string
  default     = "devops"
}

variable "subnet_cidr" {
  description = "Primary CIDR range for GKE nodes."
  type        = string
  default     = "10.10.0.0/20"
}

variable "pods_cidr" {
  description = "Secondary CIDR range for Kubernetes Pods."
  type        = string
  default     = "10.20.0.0/16"
}

variable "services_cidr" {
  description = "Secondary CIDR range for Kubernetes Services."
  type        = string
  default     = "10.30.0.0/20"
}

variable "master_ipv4_cidr" {
  description = "Non-overlapping /28 CIDR range for the GKE control plane."
  type        = string
  default     = "172.16.0.0/28"
}

variable "master_authorized_networks" {
  description = "CIDR ranges permitted to access the public GKE control-plane endpoint."
  type = list(object({
    cidr_block   = string
    display_name = string
  }))

  validation {
    condition     = length(var.master_authorized_networks) > 0
    error_message = "Provide at least one administrator or CI CIDR for master_authorized_networks."
  }
}

variable "node_count" {
  description = "Number of nodes in the primary node pool."
  type        = number
  default     = 1
}

variable "node_machine_type" {
  description = "Compute Engine machine type for cluster nodes."
  type        = string
  default     = "e2-medium"
}

variable "deletion_protection" {
  description = "Prevent accidental deletion of the GKE cluster. Set false only for disposable environments."
  type        = bool
  default     = true
}
