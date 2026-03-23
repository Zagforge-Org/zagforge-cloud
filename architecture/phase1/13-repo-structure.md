# Zagforge — Repository Structure [Phase 1]

Multi-module Go monorepo with `go.work` bridging services and shared code. Each service is its own Go module with independent `Dockerfile`, `Dockerfile.dev`, `.air.toml`, and `Makefile`. Shared Go code lives at `shared/go/` to leave room for non-Go shared assets as the platform grows:

- `shared/go/` — Go library (logger, config, server, provider interface)
- `shared/proto/` — Protobuf definitions (future: gRPC between services)
- `shared/schemas/` — JSON schemas for snapshot format validation (future: used by non-Go consumers)

```
zagforge-platform/
├── go.work                            # Bridges api, worker, shared/go modules
├── go.work.sum
├── .env.example                       # Reference template (no real secrets — use Doppler)
├── docker-compose.dev.yaml            # Dev: hot reload, volume mounts, local infra
├── docker-compose.yaml                # Prod-like: built images, no volumes
├── Makefile                           # Root: compose-dev, compose, stop, test-integration
│
├── api/                               # API service (own go.mod)
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile                     # Production multi-stage build
│   ├── Dockerfile.dev                 # Dev: Air hot reload + go.work
│   ├── .air.toml
│   ├── .dockerignore
│   │                                  # (migrations use Doppler: doppler run -- make migrate-up)
│   ├── sqlc.yaml                      # sqlc config (points to db/queries/, outputs to internal/db/)
│   ├── Makefile                       # build, run, test, migrate-*, sqlc
│   ├── cmd/
│   │   └── main.go                    # API entry point
│   ├── db/
│   │   ├── migrations/
│   │   │   ├── 000001_initial.up.sql
│   │   │   └── 000001_initial.down.sql
│   │   └── queries/
│   │       ├── organizations.sql
│   │       ├── repositories.sql
│   │       ├── jobs.sql
│   │       └── snapshots.sql
│   └── internal/
│       ├── config/
│       │   └── config.go              # caarlos0/env struct-based config
│       ├── db/                        # sqlc generated output
│       │   ├── db.go
│       │   ├── models.go
│       │   ├── querier.go
│       │   ├── organizations.sql.go
│       │   ├── repositories.sql.go
│       │   ├── jobs.sql.go
│       │   └── snapshots.sql.go
│       ├── handler/
│       │   ├── webhooks.go            # GitHub webhook handler
│       │   ├── jobs.go                # Job lifecycle endpoints
│       │   ├── snapshots.go           # Snapshot retrieval
│       │   └── watchdog.go            # Timeout handler
│       ├── middleware/
│       │   ├── clerk.go               # Clerk JWT validation
│       │   ├── internal_auth.go       # Signed token validation
│       │   ├── oidc.go                # GCP OIDC token validation
│       │   └── ratelimit.go           # Redis rate limiting middleware
│       └── engine/
│           ├── orchestrator.go        # Cloud Tasks job submission
│           └── dedup.go               # Job deduplication logic
│
├── worker/                            # Worker service (own go.mod)
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile                     # Production multi-stage build (includes git + zigzag)
│   ├── Dockerfile.dev                 # Dev: Air hot reload + go.work + ZIGZAG_MOCK
│   ├── .air.toml
│   ├── .dockerignore
│   ├── Makefile                       # build, run, test
│   ├── cmd/
│   │   └── main.go                    # Cloud Run Job entry point
│   └── internal/
│       ├── config/
│       │   └── config.go
│       └── runner/
│           └── runner.go              # Clone → Zigzag → Upload → Callback
│
├── shared/                            # Shared assets (expandable)
│   └── go/                            # Shared Go library (own go.mod)
│       ├── go.mod
│       ├── go.sum
│       ├── config/
│       │   └── config.go              # Shared config loading utilities
│       ├── logger/
│       │   └── logger.go              # Structured logging (zap)
│       ├── server/
│       │   └── server.go              # HTTP server with graceful shutdown
│       ├── provider/
│       │   ├── provider.go            # Provider interface
│       │   └── github/
│       │       ├── webhook.go         # HMAC validation, event parsing
│       │       ├── clone.go           # Shallow clone
│       │       └── app.go             # GitHub App installation
│       └── storage/
│           └── gcs.go                 # GCS upload/download
│
├── terraform/                         # [Phase 3]
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── backend.tf
│   ├── envs/
│   │   ├── dev.tfvars
│   │   ├── staging.tfvars
│   │   └── prod.tfvars
│   └── modules/
│       ├── networking/
│       ├── database/
│       ├── redis/
│       ├── storage/
│       ├── api/
│       ├── worker/
│       ├── queue/
│       ├── scheduler/
│       ├── registry/
│       └── secrets/
│
└── .github/                           # [Phase 4]
    └── workflows/
        ├── ci.yml
        ├── deploy-api.yml
        └── deploy-worker.yml
```
