terraform{
  required_providers{
    google = {
      source = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region = var.project_region
  access_token = var.access_token
}

resource "google_storage_bucket" "my_first_bucket" {
  name = "${var.project_id}-${var.resouce_name}"  
  location = var.location
  force_destroy = true
}
