variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
}

variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

# GitHub
variable "github_repo" {
  description = "GitHub repository in owner/repo format"
  type        = string
}

variable "github_app_id" {
  description = "GitHub App ID (non-sensitive)"
  type        = string
}

variable "github_app_slug" {
  description = "GitHub App slug for install URL"
  type        = string
}

# API URL (for worker callback — breaks circular dep between api/worker modules)
variable "api_url" {
  description = "API Cloud Run URL for worker callbacks"
  type        = string
  default     = ""
}

# Auth URL (used as JWT issuer and OAuth callback base)
variable "auth_url" {
  description = "Auth Cloud Run URL (e.g. https://zagforge-auth-xxx.run.app)"
  type        = string
  default     = ""
}

# CORS
variable "cors_allowed_origins" {
  description = "Comma-separated allowed origins for CORS"
  type        = string
  default     = ""
}

# Database
variable "database_provider" {
  description = "Database provider: neon (free tier) or cloudsql (staging/prod)"
  type        = string
  default     = "neon"
}

variable "cloud_sql_tier" {
  description = "Cloud SQL machine tier (when database_provider = cloudsql)"
  type        = string
  default     = "db-custom-2-8192"
}

# Redis
variable "redis_provider" {
  description = "Redis provider: upstash (free tier) or memorystore (prod)"
  type        = string
  default     = "upstash"
}

variable "redis_memory_gb" {
  description = "Redis memory size in GB (only used with memorystore)"
  type        = number
  default     = 1
}

# API scaling
variable "api_min_instances" {
  description = "Minimum API Cloud Run instances"
  type        = number
  default     = 0
}

variable "api_max_instances" {
  description = "Maximum API Cloud Run instances"
  type        = number
  default     = 2
}

# Auth scaling
variable "auth_min_instances" {
  description = "Minimum Auth Cloud Run instances (1 in prod — no cold starts on auth)"
  type        = number
  default     = 0
}

variable "auth_max_instances" {
  description = "Maximum Auth Cloud Run instances"
  type        = number
  default     = 2
}

# Worker scaling
variable "worker_cpu" {
  description = "Worker CPU allocation"
  type        = string
  default     = "1"
}

variable "worker_memory" {
  description = "Worker memory allocation"
  type        = string
  default     = "2Gi"
}

variable "worker_max_instances" {
  description = "Maximum worker instances"
  type        = number
  default     = 2
}

variable "worker_timeout" {
  description = "Worker request timeout"
  type        = string
  default     = "900s"
}

# Queue
variable "queue_max_concurrent" {
  description = "Max concurrent task dispatches"
  type        = number
  default     = 3
}

variable "queue_max_per_second" {
  description = "Max task dispatches per second"
  type        = number
  default     = 1
}

# Scheduler
variable "watchdog_schedule" {
  description = "Cron schedule for watchdog"
  type        = string
  default     = "*/30 * * * *"
}

# Networking
variable "cloud_armor_enabled" {
  description = "Enable Cloud Armor WAF (off in dev, on in prod)"
  type        = bool
  default     = false
}

variable "domain" {
  description = "Custom domain for the API (e.g. api.zagforge.com)"
  type        = string
  default     = ""
}

# Secrets are managed by Doppler — no Terraform variables needed.
# Injected at deploy time via: doppler run -- gcloud run services update ...
