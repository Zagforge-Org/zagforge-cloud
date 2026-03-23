# Zagforge — Deployment Operations [Phase 4]

Deployment is split across three tools, each owning a distinct concern:

| Tool | Owns | Frequency |
|---|---|---|
| **Terraform** | Infrastructure shape — service exists, IAM, scaling config, env var bindings, networking | Rare (infra changes) |
| **GitHub Actions** | Automated deploys — build image, push to Artifact Registry, update Cloud Run image tag | Every merge to `main` |
| **Makefile (`gcloud`)** | Manual deploys, rollbacks, promotions, debugging | On-demand |

**The critical boundary:** Terraform manages the Cloud Run service *definition* but uses `lifecycle { ignore_changes = [template[0].containers[0].image] }` so it does not fight with GitHub Actions over the image tag. Terraform says "this service exists with these permissions." GitHub Actions says "run this specific commit."

---

## Terraform × Cloud Run Boundary

```hcl
# terraform/modules/api/main.tf

resource "google_cloud_run_v2_service" "api" {
  name     = "zagforge-api"
  location = var.region

  template {
    scaling {
      min_instance_count = var.api_min_instances
      max_instance_count = var.api_max_instances
    }

    containers {
      # Initial image — GitHub Actions updates this on every deploy.
      # Terraform does NOT manage the image tag after first apply.
      image = "${var.region}-docker.pkg.dev/${var.project_id}/zagforge/api:initial"

      env {
        name  = "APP_ENV"
        value = var.environment
      }
      
      env {
        name  = "PORT"
        value = "8080"
      }

      # Secrets from Secret Manager
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }
      # ... other secret bindings
    }

    service_account = google_service_account.api.email
  }

  # CRITICAL: Let GitHub Actions own the image tag.
  # Without this, `terraform apply` would revert to "initial" on every run.
  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
    ]
  }
}
```

---

## Automated Deploy Flow (GitHub Actions)

Already specced in `10-cicd.md`. The flow is:

```
merge to main
  → GitHub Actions triggers (path filter: api/**, shared/**)
  → Build Docker image tagged with commit SHA
  → Push to Artifact Registry
  → `gcloud run deploy` updates the image tag
  → Cloud Run creates a new revision
  → Traffic migrates to the new revision (zero-downtime)
```

Each deploy creates a new **Cloud Run revision**. Old revisions are kept (Cloud Run retains them automatically). This is the foundation for instant rollbacks.

---

## Manual Deploy & Rollback — Makefile Targets

Add to the **root `Makefile`**:

```makefile
# ─── Config ───────────────────────────────────────────────────────
PROJECT_ID   ?= zagforge-prod
REGION       ?= us-central1
REGISTRY     := $(REGION)-docker.pkg.dev/$(PROJECT_ID)/zagforge

# ─── Image Management ────────────────────────────────────────────

# Build and push a specific service image tagged with current git SHA
# Usage: make image-push SERVICE=api
image-push:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make image-push SERVICE=api)
endif
	gcloud builds submit \
		--tag $(REGISTRY)/$(SERVICE):$(shell git rev-parse HEAD) \
		--dockerfile $(SERVICE)/Dockerfile \
		--project $(PROJECT_ID)

# ─── Deploy ───────────────────────────────────────────────────────

# Deploy a specific commit to Cloud Run
# Usage: make deploy SERVICE=api COMMIT=abc123f
#        make deploy SERVICE=api  (defaults to HEAD)
deploy:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make deploy SERVICE=api COMMIT=abc123f)
endif
	$(eval COMMIT ?= $(shell git rev-parse HEAD))
	@echo "Deploying $(SERVICE) at commit $(COMMIT) to $(PROJECT_ID)..."
	gcloud run deploy zagforge-$(SERVICE) \
		--image $(REGISTRY)/$(SERVICE):$(COMMIT) \
		--region $(REGION) \
		--project $(PROJECT_ID)

# ─── Rollback ─────────────────────────────────────────────────────

# List recent revisions for a service (pick one to roll back to)
# Usage: make revisions SERVICE=api
revisions:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make revisions SERVICE=api)
endif
	gcloud run revisions list \
		--service zagforge-$(SERVICE) \
		--region $(REGION) \
		--project $(PROJECT_ID) \
		--sort-by ~creationTimestamp \
		--limit 10

# Instant rollback — shift 100% traffic to a previous revision
# Usage: make rollback SERVICE=api REVISION=zagforge-api-00042-abc
rollback:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make rollback SERVICE=api REVISION=zagforge-api-00042-abc)
endif
ifndef REVISION
	$(error REVISION is not set. Run `make revisions SERVICE=api` to list them.)
endif
	@echo "Rolling back $(SERVICE) to revision $(REVISION)..."
	gcloud run services update-traffic zagforge-$(SERVICE) \
		--to-revisions $(REVISION)=100 \
		--region $(REGION) \
		--project $(PROJECT_ID)

# ─── Canary / Traffic Splitting ───────────────────────────────────

# Send a percentage of traffic to the latest revision (canary)
# Usage: make canary SERVICE=api PERCENT=10
canary:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make canary SERVICE=api PERCENT=10)
endif
ifndef PERCENT
	$(error PERCENT is not set. Usage: make canary SERVICE=api PERCENT=10)
endif
	@echo "Routing $(PERCENT)% traffic to latest revision of $(SERVICE)..."
	gcloud run services update-traffic zagforge-$(SERVICE) \
		--to-latest \
		--to-percentage $(PERCENT) \
		--region $(REGION) \
		--project $(PROJECT_ID)

# ─── Status ───────────────────────────────────────────────────────

# Show current traffic split and active revisions
# Usage: make status SERVICE=api
status:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make status SERVICE=api)
endif
	gcloud run services describe zagforge-$(SERVICE) \
		--region $(REGION) \
		--project $(PROJECT_ID) \
		--format "yaml(status.traffic)"

# Show recent logs for a service
# Usage: make logs SERVICE=api
logs:
ifndef SERVICE
	$(error SERVICE is not set. Usage: make logs SERVICE=api)
endif
	gcloud logging read \
		"resource.type=cloud_run_revision AND resource.labels.service_name=zagforge-$(SERVICE)" \
		--project $(PROJECT_ID) \
		--limit 50 \
		--format "table(timestamp, severity, textPayload)"
```

---

## Rollback Playbook

**Scenario: bad deploy reached production.**

```bash
# 1. Check what's currently serving
make status SERVICE=api

# 2. List recent revisions to find the last good one
make revisions SERVICE=api

# 3. Instant rollback — shifts traffic, no rebuild needed
make rollback SERVICE=api REVISION=zagforge-api-00042-abc

# 4. Verify
make status SERVICE=api
make logs SERVICE=api
```

Time to rollback: **< 30 seconds**. No image rebuild. Cloud Run just shifts the load balancer to the old revision which is already warm.

---

## Canary Deploys

For high-risk changes, deploy the new image but only route a fraction of traffic to it:

```bash
# Deploy the new image (creates a revision but sends 0% traffic)
make deploy SERVICE=api COMMIT=abc123f

# Route 10% of traffic to the new revision
make canary SERVICE=api PERCENT=10

# Monitor logs and error rates
make logs SERVICE=api

# If healthy, promote to 100%
gcloud run services update-traffic zagforge-api \
  --to-latest --region us-central1 --project zagforge-prod

# If broken, rollback to the previous revision
make rollback SERVICE=api REVISION=zagforge-api-00041-xyz
```

---

## Staging → Production Promotion

Staging and production use the **same Docker images** (same Artifact Registry, same commit SHA tags). Promotion means deploying an already-tested image to the prod Cloud Run service:

```bash
# The image was already built and tested in staging.
# Promote by deploying the same SHA to prod.
make deploy SERVICE=api COMMIT=abc123f PROJECT_ID=zagforge-prod
```

No rebuild. The image is identical. Only the Cloud Run service and its env vars (which point to prod DB, prod secrets) differ — those are managed by Terraform per environment.

---

## Migrations in Deploy

Database migrations are **not automatic** on deploy. They run as a manual step before deploying the new code:

```bash
# 1. Run migration against staging
cd api && doppler run --config staging -- make migrate-up

# 2. Deploy code that expects the new schema
make deploy SERVICE=api COMMIT=abc123f PROJECT_ID=zagforge-staging

# 3. Verify in staging, then repeat for prod
cd api && doppler run --config prod -- make migrate-up CONFIRM_PROD=true
make deploy SERVICE=api COMMIT=abc123f PROJECT_ID=zagforge-prod
```

**Why not automatic?** Migrations that drop columns or change types can break the currently-running code. Running them manually lets you sequence: deploy migration → verify → deploy code. For backwards-compatible migrations (add column, add table), the order doesn't matter.

---

## What This Spec Does NOT Cover

- Blue/green deployments (Cloud Run's revision model is sufficient)
- GitOps / ArgoCD (overkill for two services)
- Deployment approval gates (GitHub Actions `environment` protection rules handle this)
- Automated rollback on error rate spike (Phase 2+ — requires Cloud Monitoring alerting)
