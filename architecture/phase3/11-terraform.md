# Zagforge — Terraform (Infrastructure-as-Code) [Phase 3]

Terraform owns infrastructure shape — service definitions, IAM, scaling, networking. It does **not** own the Cloud Run image tag (that's GitHub Actions' job). See `14-deployment-ops.md` for how Terraform and deploys interact.

## Directory Structure

```
terraform/
├── main.tf                 # Root — composes modules
├── variables.tf            # Input variables
├── outputs.tf              # Output values
├── backend.tf              # GCS remote state
├── envs/
│   ├── dev.tfvars
│   ├── staging.tfvars
│   └── prod.tfvars
└── modules/
    ├── networking/
    │   ├── main.tf          # Load balancer, Cloud Armor, SSL cert, DNS
    │   ├── variables.tf
    │   └── outputs.tf
    ├── database/
    │   ├── main.tf          # Neon Postgres (dev) / Cloud SQL (prod), IAM
    │   ├── variables.tf
    │   └── outputs.tf
    ├── redis/
    │   ├── main.tf          # Memorystore instance
    │   ├── variables.tf
    │   └── outputs.tf
    ├── storage/
    │   ├── main.tf          # GCS bucket, IAM, lifecycle rules
    │   ├── variables.tf
    │   └── outputs.tf
    ├── api/
    │   ├── main.tf          # Cloud Run service, IAM, env vars, secrets
    │   ├── variables.tf
    │   └── outputs.tf
    ├── worker/
    │   ├── main.tf          # Cloud Run Job, IAM, env vars, secrets
    │   ├── variables.tf
    │   └── outputs.tf
    ├── queue/
    │   ├── main.tf          # Cloud Tasks queue config
    │   ├── variables.tf
    │   └── outputs.tf
    ├── scheduler/
    │   ├── main.tf          # Cloud Scheduler job (watchdog)
    │   ├── variables.tf
    │   └── outputs.tf
    ├── registry/
    │   ├── main.tf          # Artifact Registry repo
    │   ├── variables.tf
    │   └── outputs.tf
    └── secrets/
        ├── main.tf          # Secret Manager secrets + IAM bindings
        ├── variables.tf
        └── outputs.tf
```

## State Management

Remote state in a GCS bucket (`zagforge-terraform-state`), one state file per environment:

```hcl
terraform {
  backend "gcs" {
    bucket = "zagforge-terraform-state"
    prefix = "env/${var.environment}"
  }
}
```

## Environment Tfvars

`envs/dev.tfvars`:
```hcl
environment        = "dev"
project_id         = "zagforge-dev"
region             = "us-central1"
database_provider  = "neon"         # Free tier, no Cloud SQL cost
neon_project_id    = "zagforge-dev"
redis_memory_gb    = 1
api_min_instances  = 0     # scale to zero in dev
api_max_instances  = 2
cloud_armor_enabled = false  # skip in dev
```

`envs/prod.tfvars`:
```hcl
environment        = "prod"
project_id         = "zagforge-prod"
region             = "us-central1"
database_provider  = "cloudsql"     # Upgrade from Neon for production
cloud_sql_tier     = "db-custom-2-8192"
redis_memory_gb    = 2
api_min_instances  = 1     # always-on in prod
api_max_instances  = 10
cloud_armor_enabled = true
```

## Key Module: Networking

```hcl
# modules/networking/main.tf

resource "google_compute_global_address" "api" {
  name = "zagforge-api-ip"
}

resource "google_compute_managed_ssl_certificate" "api" {
  name = "zagforge-api-cert"
  managed {
    domains = ["api.zagforge.com"]
  }
}

resource "google_compute_security_policy" "api" {
  count = var.cloud_armor_enabled ? 1 : 0
  name  = "zagforge-api-policy"

  # Default rate limit
  rule {
    action   = "rate_based_ban"
    priority = 1000
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      rate_limit_threshold {
        count        = 1000
        interval_sec = 60
      }
    }
  }

  # OWASP CRS
  rule {
    action   = "deny(403)"
    priority = 2000
    match {
      expr {
        expression = "evaluatePreconfiguredWaf('sqli-v33-stable') || evaluatePreconfiguredWaf('xss-v33-stable')"
      }
    }
  }
}
```
