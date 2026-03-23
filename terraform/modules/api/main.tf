resource "google_service_account" "api" {
  account_id   = "${var.name_prefix}-api"
  display_name = "Zagforge API service account"
}

resource "google_cloud_run_v2_service" "api" {
  name     = "${var.name_prefix}-api"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.api.email

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      # Placeholder image — GitHub Actions owns the actual image tag.
      image = "us-docker.pkg.dev/cloudrun/container/hello"

      ports {
        container_port = 8080
      }

      # --- Non-sensitive config (managed by Terraform) ---
      env {
        name  = "APP_ENV"
        value = var.environment
      }
      env {
        name  = "PORT"
        value = "8080"
      }
      env {
        name  = "GITHUB_APP_ID"
        value = var.github_app_id
      }
      env {
        name  = "GITHUB_APP_SLUG"
        value = var.github_app_slug
      }
      env {
        name  = "GCS_BUCKET"
        value = var.gcs_bucket
      }
      env {
        name  = "CLOUD_TASKS_PROJECT"
        value = var.cloud_tasks_project
      }
      env {
        name  = "CLOUD_TASKS_LOCATION"
        value = var.cloud_tasks_location
      }
      env {
        name  = "CLOUD_TASKS_QUEUE"
        value = var.cloud_tasks_queue
      }
      env {
        name  = "CLOUD_TASKS_WORKER_URL"
        value = var.cloud_tasks_worker_url
      }
      env {
        name  = "CLOUD_TASKS_SERVICE_ACCOUNT"
        value = var.cloud_tasks_service_account
      }
      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = var.cors_allowed_origins
      }

      # --- Secrets (managed by Doppler, injected at deploy time) ---
      # The following env vars are set via `doppler run -- gcloud run services update`:
      #   DATABASE_URL, REDIS_URL, GITHUB_APP_PRIVATE_KEY,
      #   GITHUB_APP_WEBHOOK_SECRET, HMAC_SIGNING_KEY, CLERK_SECRET_KEY,
      #   WATCHDOG_SECRET, ENCRYPTION_KEY_BASE64, CLI_API_KEY

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }

      startup_probe {
        http_get {
          path = "/readyz"
        }
        initial_delay_seconds = 5
        period_seconds        = 3
        failure_threshold     = 10
      }

      liveness_probe {
        http_get {
          path = "/readyz"
        }
        period_seconds = 30
      }
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
      template[0].containers[0].env,
    ]
  }
}

# Allow unauthenticated access (public API)
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.api.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}
