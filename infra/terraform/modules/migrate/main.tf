# Cloud Run Job: API migrations
resource "google_cloud_run_v2_job" "migrate" {
  name     = "${var.name_prefix}-migrate"
  location = var.region

  template {
    template {
      service_account = var.api_service_account

      containers {
        # Placeholder — deploy workflow pushes the real image.
        image = "us-docker.pkg.dev/cloudrun/container/hello"

        # DATABASE_URL injected by Doppler at deploy time.
        args = ["-database", "$(DATABASE_URL)", "up"]
      }

      max_retries = 0
      timeout     = "300s"
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].template[0].containers[0].image,
      template[0].template[0].containers[0].env,
    ]
  }
}

# Cloud Run Job: Auth migrations
resource "google_cloud_run_v2_job" "auth_migrate" {
  name     = "${var.name_prefix}-auth-migrate"
  location = var.region

  template {
    template {
      service_account = var.auth_service_account

      containers {
        # Placeholder — deploy workflow pushes the real image.
        image = "us-docker.pkg.dev/cloudrun/container/hello"

        # AUTH_DATABASE_URL injected by Doppler at deploy time.
        args = ["-database", "$(AUTH_DATABASE_URL)", "up"]
      }

      max_retries = 0
      timeout     = "300s"
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].template[0].containers[0].image,
      template[0].template[0].containers[0].env,
    ]
  }
}
