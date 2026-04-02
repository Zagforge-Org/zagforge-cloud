# GCP Migration: zagforge -> zagforge-org

## Status

Project: `zagforge-org` (project number: 720047738006)
Region: `us-central1`
Account: `peternelanze@gmail.com` (roles: Editor, Viewer)

## Completed

- [x] GCP project created
- [x] gcloud CLI pointed at zagforge-org
- [x] Terraform state bucket (`zagforge-org-terraform-state`) created
- [x] `terraform init` successful
- [x] All required GCP APIs enabled
- [x] Doppler secrets verified and up to date (dev config)
- [x] Google OAuth credentials created and added to Doppler
- [x] Artifact Registry cleanup policy added (keep 5 recent, delete after 7 days)
- [x] Cost optimizations applied (worker sizing, queue limits, scheduler frequency)
- [x] Terraform apply (partial) — services created:
  - Artifact Registry
  - Cloud Run: API, Auth, Worker (with service accounts)
  - Cloud Tasks queue
  - Cloud Scheduler watchdog (every 30 min)
  - GCS snapshots bucket
  - Migrate jobs (API + Auth)

## Blocked — needs Owner to grant permissions

### 1. Cloud Run IAM bindings (need `roles/run.admin`)
- API public access (allUsers invoker)
- Auth public access (allUsers invoker)
- Worker Cloud Tasks invoker

### 2. WIF module (need `roles/resourcemanager.projectIamAdmin` + `roles/iam.workloadIdentityPoolAdmin`)
- WIF pool + provider
- GitHub Actions service account
- IAM bindings for CI/CD

### Once permissions are granted, run:
```bash
# 1. Uncomment WIF module in main.tf and outputs.tf
# 2. Apply
terraform apply -var-file=envs/dev.tfvars

# 3. Grab WIF outputs and update GitHub secrets
terraform output wif_provider
terraform output wif_service_account
gh secret set WIF_PROVIDER --body "<value>"
gh secret set WIF_SERVICE_ACCOUNT --body "<value>"
```

## Still to verify

- [ ] `DATABASE_URL` / `AUTH_DATABASE_URL` in Doppler — do they point to valid Neon instances?
- [ ] `REDIS_URL` in Doppler — does it point to a valid Upstash instance?
- [ ] `DOPPLER_SERVICE_TOKEN` in GitHub secrets — does it work for the current Doppler project?
- [ ] GitHub repo in dev.tfvars (`LegationPro/zagforge-cloud`) — is this correct?

## Cost controls (dev environment)

| Setting | Value |
|---------|-------|
| Worker CPU / Memory | 1 CPU / 2Gi |
| Worker max instances | 2 |
| Queue concurrent dispatches | 3 |
| Queue dispatches/sec | 1 |
| Queue retry duration cap | 600s |
| Watchdog schedule | every 30 min |
| Artifact Registry | auto-cleanup: keep 5, delete > 7 days |
| All Cloud Run min instances | 0 (scale to zero) |
