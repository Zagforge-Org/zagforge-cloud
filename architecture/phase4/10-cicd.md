# Zagforge — CI/CD (GitHub Actions) [Phase 4]

## Pipeline Structure

```
.github/
└── workflows/
    ├── ci.yml          # Runs on all PRs
    ├── deploy-api.yml  # Deploys API on merge to main
    └── deploy-worker.yml  # Deploys worker on merge to main
```

## CI Pipeline (`ci.yml`)

Triggers on: pull request to `main`

Multi-module aware: uses `go.work` at the repo root so `go vet` and `go test` resolve all modules. **All three modules are always tested together** — a change in `shared/go/` that breaks `worker/` will be caught even if only `api/` files changed, because the workspace resolves everything.

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go vet ./api/... ./worker/... ./shared/go/...
      - uses: golangci/golangci-lint-action@v6

  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_DB: zagforge_test
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test -race -coverprofile=coverage.out ./api/... ./worker/... ./shared/go/...
      - uses: codecov/codecov-action@v4

  test-integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker-compose -f docker-compose.dev.yaml up -d --build
      - run: |
          # Wait for API to be healthy
          for i in $(seq 1 30); do
            curl -sf http://localhost:8080/healthz && break || sleep 2
          done
      - run: go test -tags=integration -race ./api/... ./worker/...
      - run: docker-compose -f docker-compose.dev.yaml down

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -f api/Dockerfile -t api-test .
      - run: docker build -f worker/Dockerfile -t worker-test .

  sqlc-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: sqlc-dev/setup-sqlc@v4
      - run: cd api && sqlc diff  # Fails if generated code is out of date
```

## Deploy Pipeline (`deploy-api.yml`)

Triggers on: push to `main` (paths: `api/**`, `shared/**`)

Build context is the repo root so the Dockerfile can resolve `shared/go/` via `go mod edit -replace`.

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - id: auth
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.WIF_PROVIDER }}
          service_account: ${{ secrets.WIF_SERVICE_ACCOUNT }}

      - uses: google-github-actions/setup-gcloud@v2

      - name: Build and push to Artifact Registry
        run: |
          gcloud builds submit \
            --tag ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/zagforge/api:${{ github.sha }} \
            --dockerfile api/Dockerfile

      - name: Deploy to Cloud Run
        uses: google-github-actions/deploy-cloudrun@v2
        with:
          service: zagforge-api
          image: ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/zagforge/api:${{ github.sha }}
          region: ${{ env.REGION }}
```

## Deploy Pipeline (`deploy-worker.yml`)

Triggers on: push to `main` (paths: `worker/**`, `shared/**`)

Same pattern as `deploy-api.yml` but builds `worker/Dockerfile` and deploys to the `zagforge-worker` Cloud Run Job.

**Auth:** Workload Identity Federation (no service account keys stored in GitHub). The GitHub Actions OIDC token exchanges for a GCP access token.

**Deploy strategy:** Cloud Run handles zero-downtime deployments natively (traffic migration to new revision). For manual deploys, rollbacks, canary routing, and staging→prod promotion, see `14-deployment-ops.md`.

**Shared code changes:** Both deploy pipelines trigger when `shared/**` changes, since shared code is compiled into both service binaries. The CI pipeline always tests all three modules together regardless of what changed, so breakage from shared changes is caught before merge.
