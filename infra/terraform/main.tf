terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

locals {
  name_prefix = "zagforge"
}

# --- Artifact Registry ---
module "registry" {
  source = "./modules/registry"

  project_id  = var.project_id
  region      = var.region
  name_prefix = local.name_prefix
}

# --- Secrets managed by Doppler (no GCP Secret Manager) ---
# Secrets are injected into Cloud Run at deploy time via:
#   doppler run -- gcloud run services update ... --update-env-vars ...
# See: https://docs.doppler.com/docs/google-cloud-run

# --- GCS Storage ---
module "storage" {
  source = "./modules/storage"

  project_id             = var.project_id
  region                 = var.region
  name_prefix            = local.name_prefix
  api_service_account    = module.api.service_account_email
  worker_service_account = module.worker.service_account_email
}

# --- Database ---
# Cloud SQL is only provisioned for staging/prod (database_provider = "cloudsql").
# Dev uses Neon (free tier) — DATABASE_URL and AUTH_DATABASE_URL managed via Doppler.
# Cloud SQL creates two databases: zagforge (API) and zagforge_auth (Auth).
module "database" {
  source = "./modules/database"

  project_id        = var.project_id
  region            = var.region
  name_prefix       = local.name_prefix
  database_provider = var.database_provider
  cloud_sql_tier    = var.cloud_sql_tier
}

# --- Redis ---
# Dev uses Upstash (free tier) — REDIS_URL managed via Doppler.
# Prod uses Memorystore.
module "redis" {
  source = "./modules/redis"

  project_id     = var.project_id
  region         = var.region
  name_prefix    = local.name_prefix
  redis_provider = var.redis_provider
  memory_gb      = var.redis_memory_gb
}

# --- Cloud Tasks Queue ---
module "queue" {
  source = "./modules/queue"

  project_id                = var.project_id
  region                    = var.region
  name_prefix               = local.name_prefix
  max_concurrent_dispatches = var.queue_max_concurrent
  max_dispatches_per_second = var.queue_max_per_second
}

# --- Auth (Cloud Run Service) ---
module "auth" {
  source = "./modules/auth"

  project_id              = var.project_id
  region                  = var.region
  name_prefix             = local.name_prefix
  environment             = var.environment
  min_instances           = var.auth_min_instances
  max_instances           = var.auth_max_instances
  jwt_issuer              = var.auth_url
  oauth_callback_base_url = var.auth_url
  cors_allowed_origins    = var.cors_allowed_origins
}

# --- API (Cloud Run Service) ---
module "api" {
  source = "./modules/api"

  project_id                  = var.project_id
  region                      = var.region
  name_prefix                 = local.name_prefix
  min_instances               = var.api_min_instances
  max_instances               = var.api_max_instances
  environment                 = var.environment
  github_app_id               = var.github_app_id
  github_app_slug             = var.github_app_slug
  gcs_bucket                  = module.storage.bucket_name
  cloud_tasks_project         = var.project_id
  cloud_tasks_location        = var.region
  cloud_tasks_queue           = module.queue.queue_name
  cloud_tasks_worker_url      = module.worker.url
  cloud_tasks_service_account = module.api.service_account_email
  cors_allowed_origins        = var.cors_allowed_origins
}

# --- Worker (Cloud Run Service) ---
module "worker" {
  source = "./modules/worker"

  project_id    = var.project_id
  region        = var.region
  name_prefix   = local.name_prefix
  environment   = var.environment
  github_app_id = var.github_app_id
  gcs_bucket    = module.storage.bucket_name
  api_url       = var.api_url
  cpu           = var.worker_cpu
  memory        = var.worker_memory
  max_instances = var.worker_max_instances
  timeout       = var.worker_timeout
}

# --- Migration Jobs (Cloud Run Jobs) ---
module "migrate" {
  source = "./modules/migrate"

  project_id           = var.project_id
  region               = var.region
  name_prefix          = local.name_prefix
  api_service_account  = module.api.service_account_email
  auth_service_account = module.auth.service_account_email
}

# --- Cloud Scheduler (Watchdog) ---
module "scheduler" {
  source = "./modules/scheduler"

  project_id          = var.project_id
  region              = var.region
  name_prefix         = local.name_prefix
  api_url             = module.api.url
  api_service_account = module.api.service_account_email
  watchdog_schedule   = var.watchdog_schedule
}

# --- Workload Identity Federation (GitHub Actions → GCP) ---
# Skipped: requires roles/owner or projectIamAdmin permissions.
# Uncomment once IAM permissions are granted.
# module "wif" {
#   source = "./modules/wif"
#
#   project_id  = var.project_id
#   github_repo = var.github_repo
# }

# --- Networking (LB + Cloud Armor) ---
module "networking" {
  source = "./modules/networking"

  project_id          = var.project_id
  name_prefix         = local.name_prefix
  cloud_armor_enabled = var.cloud_armor_enabled
  domain              = var.domain
  api_cloud_run_name  = module.api.service_name
  region              = var.region
}
