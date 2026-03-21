# Zagforge — Local Development [Phase 1]

Follows the split compose pattern: `docker-compose.dev.yaml` for local development with hot reload and volume mounts, `docker-compose.yaml` for production-like builds.

---

## `go.work`

Bridges all Go modules in the monorepo. Shared Go code lives at `shared/go/` to leave room for `shared/proto/`, `shared/schemas/` etc. in future:

```go
go 1.24

use (
	./api
	./worker
	./shared/go
)
```

---

## Secrets Management — Doppler

**No `.env` files with real secrets.** Secrets are managed via [Doppler](https://www.doppler.com/), which you already use in partifly. Team members get project-level access — no secrets shared over Slack, no `.env` files to accidentally commit.

### Doppler Setup

```
Project: zagforge
├── dev           # Local development (shared across team)
├── staging       # Staging environment
└── prod          # Production (restricted access)
```

**Required secrets (managed in Doppler `dev` config):**

| Key | Description |
|---|---|
| `GITHUB_APP_ID` | GitHub App ID |
| `GITHUB_APP_PRIVATE_KEY` | GitHub App RSA private key (multiline) |
| `GITHUB_APP_WEBHOOK_SECRET` | GitHub webhook HMAC secret |
| `HMAC_SIGNING_KEY` | Job token signing key (≥32 bytes) |
| `CLERK_SECRET_KEY` | Clerk API secret key |

**Non-secret config (set directly in docker-compose, not Doppler):**

| Key | Value | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://zagforge:zagforge@postgres:5432/zagforge?sslmode=disable` | Local Postgres, not sensitive |
| `REDIS_URL` | `redis://redis:6379` | Local Redis |
| `GCS_ENDPOINT` | `http://fake-gcs:4443` | Local fake GCS |
| `GCS_BUCKET` | `zagforge-snapshots` | Bucket name |

### Onboarding a new team member

1. Install Doppler CLI: `brew install dopplerhq/cli/doppler`
2. Authenticate: `doppler login`
3. Link the project: `doppler setup` (select `zagforge` → `dev`)
4. Run: `make compose-dev` (compose uses `doppler run --` to inject secrets)

No `.env` files to copy, no secrets to share manually.

### `.env.example` (reference only)

Committed to the repo as documentation. Lists all required keys with placeholder values. Never contains real secrets:

```bash
# Managed by Doppler — this file is for reference only.
# Do NOT create a .env file with real values. Use: doppler setup

# --- Secrets (set in Doppler, injected at runtime) ---
GITHUB_APP_ID=000000
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
GITHUB_APP_WEBHOOK_SECRET=whsec_placeholder
HMAC_SIGNING_KEY=placeholder-at-least-32-bytes-long!!
CLERK_SECRET_KEY=sk_test_placeholder

# --- Non-secrets (set in docker-compose.dev.yaml) ---
# DATABASE_URL=postgres://zagforge:zagforge@postgres:5432/zagforge?sslmode=disable
# REDIS_URL=redis://redis:6379
# GCS_ENDPOINT=http://fake-gcs:4443
# GCS_BUCKET=zagforge-snapshots
```

### `.gitignore` entry

```
.env
.env.*
!.env.example
```

---

## `docker-compose.dev.yaml`

Secrets are injected by Doppler at the compose level. Non-secret config is set directly in the yaml:

```yaml
services:
  api:
    restart: "no"
    build:
      context: .
      dockerfile: api/Dockerfile.dev
    environment:
      # Non-secrets: safe to hardcode for local dev
      - APP_ENV=dev
      - APP_LOG_LEVEL=debug
      - PORT=8080
      - DATABASE_URL=postgres://zagforge:zagforge@postgres:5432/zagforge?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - GCS_ENDPOINT=http://fake-gcs:4443
      - GCS_BUCKET=zagforge-snapshots
      # Secrets: injected by Doppler via `doppler run --`
      - GITHUB_APP_WEBHOOK_SECRET
      - GITHUB_APP_PRIVATE_KEY
      - GITHUB_APP_ID
      - HMAC_SIGNING_KEY
      - CLERK_SECRET_KEY
    volumes:
      - ./shared:/app/shared
      - ./api:/app/api
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
      - fake-gcs

  worker:
    restart: "no"
    build:
      context: .
      dockerfile: worker/Dockerfile.dev
    environment:
      - APP_ENV=dev
      - APP_LOG_LEVEL=debug
      - GCS_ENDPOINT=http://fake-gcs:4443
      - GCS_BUCKET=zagforge-snapshots
      - API_CALLBACK_URL=http://api:8080
      - ZIGZAG_MOCK=true
    volumes:
      - ./shared:/app/shared
      - ./worker:/app/worker
    depends_on:
      - api

  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: zagforge
      POSTGRES_USER: zagforge
      POSTGRES_PASSWORD: zagforge
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  fake-gcs:
    image: fsouza/fake-gcs-server:latest
    ports:
      - "4443:4443"
    command: ["-scheme", "http", "-port", "4443"]

volumes:
  pgdata:
```

---

## `docker-compose.yaml`

Production-like compose (no volume mounts, no dev tools):

```yaml
services:
  api:
    build:
      context: .
      dockerfile: api/Dockerfile
    environment:
      - APP_ENV=prod
      - PORT=8080
    ports:
      - "8080:8080"

  worker:
    build:
      context: .
      dockerfile: worker/Dockerfile
    environment:
      - APP_ENV=prod
```

---

## Root `Makefile`

```makefile
.PHONY: compose-dev compose-dev-d compose compose-d stop test-integration

# Dev: Doppler injects secrets, docker-compose sets non-secrets
compose-dev:
	doppler run --project zagforge --config dev -- docker-compose -f docker-compose.dev.yaml up --build

compose-dev-d:
	doppler run --project zagforge --config dev -- docker-compose -f docker-compose.dev.yaml up --build -d

compose:
	docker-compose -f docker-compose.yaml up --build

compose-d:
	docker-compose -f docker-compose.yaml up --build -d

stop:
	docker-compose -f docker-compose.dev.yaml down
	docker-compose -f docker-compose.yaml down

test-integration:
	doppler run --project zagforge --config dev -- docker-compose -f docker-compose.dev.yaml up -d --build
	@echo "Waiting for API to be healthy..."
	@for i in $$(seq 1 30); do curl -sf http://localhost:8080/healthz && break || sleep 2; done
	go test -tags=integration -race ./api/... ./worker/...
	docker-compose -f docker-compose.dev.yaml down
```

---

## `api/Makefile`

Per-service Makefile with migration guards matching the established pattern. Migrations live inside the API service at `db/migrations/`. Secrets injected via `doppler run -- make migrate-up`:

```makefile
# DB_URL comes from Doppler (doppler run -- make migrate-up)
# or can be exported manually for standalone use.
ifndef DB_URL
$(error DB_URL is not set. Run via: doppler run -- make migrate-up)
endif

ENV ?= $(APP_ENV)
ENV ?= dev

# Prevent accidents in prod
ifeq ($(ENV),prod)
PROD_GUARD := true
endif

.PHONY: build run test migrate-* guard-prod guard-db-url sqlc

guard-db-url:
ifndef DB_URL
	$(error DB_URL is not set)
endif

guard-prod:
ifeq ($(PROD_GUARD),true)
ifndef CONFIRM_PROD
	$(error You are running migrations in PRODUCTION. Re-run with CONFIRM_PROD=true)
endif
endif

build:
	go build -o .bin/api cmd/main.go

run:
	go run cmd/main.go

test:
	go test -race ./...

sqlc:
	sqlc generate

migrate-up: guard-db-url guard-prod
	@echo "Running migrations ($(ENV))..."
	migrate -path ./db/migrations -database "$(DB_URL)" up

migrate-down: guard-db-url guard-prod
	@echo "Reverting last migration ($(ENV))..."
	migrate -path ./db/migrations -database "$(DB_URL)" down 1

migrate-status: guard-db-url
	@echo "Migration status ($(ENV))..."
	migrate -path ./db/migrations -database "$(DB_URL)" version

migrate-force: guard-db-url guard-prod
ifndef VERSION
	$(error VERSION is not set. Re-run with VERSION=<number>)
endif
	@echo "Forcing migration version to $(VERSION) ($(ENV))..."
	migrate -path ./db/migrations -database "$(DB_URL)" force $(VERSION)

migrate-create:
ifndef NAME
	$(error NAME is not set. Re-run with NAME=<migration_name>)
endif
	@echo "Creating new migration '$(NAME)' ($(ENV))..."
	migrate create -ext sql -dir ./db/migrations -seq $(NAME)
```

---

## `worker/Makefile`

```makefile
.PHONY: build run test

build:
	go build -o .bin/worker cmd/main.go

run:
	go run cmd/main.go

test:
	go test -race ./...
```

---

## Running Locally

1. Install and configure Doppler:
   ```bash
   brew install dopplerhq/cli/doppler
   doppler login
   doppler setup  # select zagforge → dev
   ```

2. Start the dev environment:
   ```bash
   make compose-dev
   ```

3. Run migrations (from `api/` directory):
   ```bash
   cd api && make migrate-up
   ```

4. The API is available at `http://localhost:8080`, with hot reload on code changes via Air volume mounts.

5. The worker runs with `ZIGZAG_MOCK=true` by default in dev — it simulates snapshot generation without running the actual Zigzag binary. To test with real Zigzag, mount the binary and unset the mock flag.

6. Run integration tests (api↔worker callback flow):
   ```bash
   make test-integration
   ```
