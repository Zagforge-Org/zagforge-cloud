resource "google_artifact_registry_repository" "main" {
  location      = var.region
  repository_id = var.name_prefix
  format        = "DOCKER"
  description   = "Docker images for ${var.name_prefix} services"

  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"

    most_recent_versions {
      keep_count = 5
    }
  }

  cleanup_policies {
    id     = "delete-old"
    action = "DELETE"

    condition {
      older_than = "604800s" # 7 days
    }
  }

  cleanup_policy_dry_run = false
}
