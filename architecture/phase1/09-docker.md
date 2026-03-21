# Zagforge — Docker [Phase 1]

Follows the multi-module monorepo pattern: each service has its own `Dockerfile.dev` (hot reload via Air) and `Dockerfile` (production multi-stage build). Shared Go code lives at `shared/go/` (leaving room for `shared/proto/`, `shared/schemas/`, etc. in future). Resolved via `go work init` in dev and `go mod edit -replace` in prod.

---

## API Service

### `api/Dockerfile.dev`

```dockerfile
FROM golang:1.24-alpine

RUN apk add --no-cache curl

RUN curl -sSfL https://raw.githubusercontent.com/air-verse/air/master/install.sh | sh -s

WORKDIR /app

COPY api ./api
COPY shared ./shared

# Initialize go.work with the module paths
RUN go work init ./api ./shared/go

WORKDIR /app/api
RUN go mod download

CMD ["air"]
```

### `api/Dockerfile`

```dockerfile
# ----------------------------
# Builder Stage
# ----------------------------
FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN apk add --no-cache git ca-certificates

WORKDIR /build-context
# Copy everything to resolve shared modules
COPY . .

WORKDIR /build-context/api
RUN go mod edit -replace github.com/LegationPro/zagforge/shared/go=../shared/go
RUN go mod download
# Build the binary to a specific, predictable path
RUN go build -trimpath -ldflags="-s -w" -o /api-bin ./cmd/main.go

# ----------------------------
# Runtime Stage
# ----------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# Use a clean directory
WORKDIR /app

# ONLY copy the binary
COPY --from=builder /api-bin ./api

ENTRYPOINT ["./api"]
```

### `api/.air.toml`

```toml
env_files = []
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = ".bin/api"
  cmd = "go build -o .bin/api cmd/main.go"
  delay = 1000
  entrypoint = [".bin/api"]
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  post_cmd = []
  pre_cmd = []
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  silent = false
  time = false

[misc]
  clean_on_exit = false

[proxy]
  app_port = 0
  app_start_timeout = 0
  enabled = false
  proxy_port = 0

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

### `api/.dockerignore`

```dockerignore
tmp
```

---

## Worker Service

### `worker/Dockerfile.dev`

```dockerfile
FROM golang:1.24-alpine

RUN apk add --no-cache curl git

RUN curl -sSfL https://raw.githubusercontent.com/air-verse/air/master/install.sh | sh -s

WORKDIR /app

COPY worker ./worker
COPY shared ./shared

# Initialize go.work with the module paths
RUN go work init ./worker ./shared/go

WORKDIR /app/worker
RUN go mod download

# Install Zigzag binary for local snapshot generation
# In dev, mount a local zigzag binary or use ZIGZAG_BIN env to override path.
# Set ZIGZAG_MOCK=true to skip actual snapshot generation during development.

CMD ["air"]
```

### `worker/Dockerfile`

```dockerfile
# ----------------------------
# Builder Stage
# ----------------------------
FROM golang:1.24-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN apk add --no-cache git ca-certificates

WORKDIR /build-context
# Copy everything to resolve shared modules
COPY . .

WORKDIR /build-context/worker
RUN go mod edit -replace github.com/LegationPro/zagforge/shared/go=../shared/go
RUN go mod download
# Build the binary to a specific, predictable path
RUN go build -trimpath -ldflags="-s -w" -o /worker-bin ./cmd/main.go

# ----------------------------
# Zigzag Stage — fetch the Zigzag binary
# ----------------------------
FROM golang:1.24-alpine AS zigzag
RUN go install github.com/LegationPro/zigzag@latest

# ----------------------------
# Runtime Stage
# ----------------------------
FROM alpine:3.21 AS runtime
RUN apk add --no-cache git ca-certificates

# Create user
RUN adduser -S -D -u 10001 appuser
USER appuser

WORKDIR /app

# Copy the worker binary
COPY --from=builder /worker-bin ./worker
# Copy the Zigzag binary
COPY --from=zigzag /go/bin/zigzag /usr/local/bin/zigzag

ENTRYPOINT ["./worker"]
```

### `worker/.air.toml`

```toml
env_files = []
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = ".bin/worker"
  cmd = "go build -o .bin/worker cmd/main.go"
  delay = 1000
  entrypoint = [".bin/worker"]
  exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  post_cmd = []
  pre_cmd = []
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  silent = false
  time = false

[misc]
  clean_on_exit = false

[proxy]
  app_port = 0
  app_start_timeout = 0
  enabled = false
  proxy_port = 0

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

### `worker/.dockerignore`

```dockerignore
tmp
```

---

## Key Decisions

- `distroless` base for the API (minimal attack surface, no shell)
- `alpine` base for the worker (needs `git` binary for cloning repos)
- `nonroot` / `appuser` in both images
- `-trimpath -ldflags="-s -w"` strips debug info and build paths for smaller, reproducible binaries
- `CGO_ENABLED=0` for fully static binaries
- `go mod edit -replace` in prod Dockerfiles resolves `shared/go/` without needing `go.work` (which is intentionally excluded from module-aware builds)
- Dev containers use `go work init` + Air for hot reload with volume mounts
- Worker prod image includes Zigzag binary via a dedicated build stage; dev uses `ZIGZAG_BIN` env override or `ZIGZAG_MOCK=true` to skip

**Image registry:** GCP Artifact Registry in the same region as Cloud Run services.
