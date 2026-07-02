resource "google_container_cluster" "primary_nodes" {
  name = "primary"
  location = var.region
  initial_node_account = 1
  deletion_protection = false
}

resource "google_container_node_pool" "primary_nodes" {
  name = "primary"
}
