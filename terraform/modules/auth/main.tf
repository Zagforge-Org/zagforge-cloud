resource "google_service_account" "auth" {
  account_id   = "${var.name_prefix}-auth"
  display_name = "Zagforge Auth service account"
}

resource "google_cloud_run_v2_service" "auth" {
  name     = "${var.name_prefix}-auth"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.auth.email

    scaling {
      min_instance_count = var.min_instances
      max_instance_count = var.max_instances
    }

    containers {
      # Placeholder image — GitHub Actions owns the actual image tag.
      image = "us-docker.pkg.dev/cloudrun/container/hello"

      ports {
        container_port = 8081
      }

      # --- Non-sensitive config (managed by Terraform) ---
      env {
        name  = "APP_ENV"
        value = var.environment
      }
      env {
        name  = "PORT"
        value = "8081"
      }
      env {
        name  = "JWT_ISSUER"
        value = var.jwt_issuer
      }
      env {
        name  = "OAUTH_CALLBACK_BASE_URL"
        value = var.oauth_callback_base_url
      }
      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = var.cors_allowed_origins
      }

      # --- Secrets (managed by Doppler, injected at deploy time) ---
      #   AUTH_DATABASE_URL, REDIS_URL, JWT_PRIVATE_KEY_BASE64, JWT_PUBLIC_KEY_BASE64,
      #   GITHUB_OAUTH_CLIENT_ID, GITHUB_OAUTH_CLIENT_SECRET,
      #   GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET,
      #   ENCRYPTION_KEY_BASE64, FRONTEND_URL

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

# Allow unauthenticated access (public auth endpoints)
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.auth.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}
