# Zagforge — Networking & Rate Limiting [Phase 3]

## Cloud Load Balancer (L7)

A global external Application Load Balancer routes traffic to Cloud Run services by path prefix:

| Path prefix | Backend service | Notes |
|---|---|---|
| `/api/v1/*` | `api` Cloud Run service | Public, Zitadel OIDC JWT auth |
| `/auth/*` | `api` Cloud Run service | GitHub App OAuth flows |
| `/internal/webhooks/*` | `api` Cloud Run service | GitHub webhook receiver |
| `/internal/jobs/*` | `api` Cloud Run service | Worker callbacks (signed token auth) |
| `/internal/watchdog/*` | `api` Cloud Run service | Cloud Scheduler (OIDC auth) |

**Phase 1 note:** Cloud Load Balancer and Cloud Armor are optional during development. Use Cloud Run's built-in HTTPS URLs directly (`*.run.app`). Add the LB + Cloud Armor when custom domains (`api.zagforge.com`) and DDoS protection are needed (saves ~$23/month in dev).

**SSL:** Managed SSL certificate via Google-managed certs for `api.zagforge.com` (when LB is enabled).

**Health checks:** HTTP health check on `/healthz` endpoint (returns 200 if DB connection is alive).

---

## Cloud Armor

Security policy attached to the load balancer backend:

**DDoS protection:**
- Adaptive protection enabled (automatic anomaly detection)
- Default rate limit: 1000 requests per IP per minute (outer layer, catches volumetric abuse)

**WAF rules:**
- OWASP ModSecurity Core Rule Set (CRS) — blocks SQL injection, XSS, etc.
- Block requests with bodies > 10MB (snapshots are read from GCS, not uploaded through the API)

**Internal endpoint protection:**
- `/internal/watchdog/*` — restrict to Cloud Scheduler's IP ranges + OIDC validation in the app
- `/internal/jobs/*` — restrict to Cloud Run Job egress IP ranges where possible, signed token validation in the app

**Geo restrictions:** None initially. Can be added per-customer in Phase 2.

---

## Rate Limiting

### Layer 1: Cloud Armor (outer)

IP-based rate limiting at the load balancer level. Stops volumetric abuse before it hits Cloud Run.

- 1000 requests per IP per minute (global default)
- Configurable per-path overrides (e.g., `/internal/webhooks/github` can have a higher limit for GitHub's webhook IPs)

### Layer 2: Application-level (inner, Redis/Memorystore)

Granular per-key and per-org rate limiting inside the Go API middleware.

**Infrastructure:** Upstash Redis (serverless, free tier) for Phase 1. Upgrade to GCP Memorystore when usage exceeds Upstash free tier (~10K requests/day). Upstash is cross-region, pay-per-request, and eliminates the ~$35/month Memorystore base cost during development.

**Key schema:**

```
ratelimit:{api_key}:{endpoint_group}:{window}
```

Example:
```
ratelimit:key_abc123:snapshots:2026-03-14T12:05
```

**Sliding window algorithm:**

```
MULTI
  INCR ratelimit:{key}:{group}:{window}
  EXPIRE ratelimit:{key}:{group}:{window} {ttl}
EXEC
```

If count exceeds limit → return HTTP 429 with `Retry-After` header.

**Default rate limits (Phase 1):**

| Endpoint group | Free tier | Notes |
|---|---|---|
| Snapshot retrieval (`GET /api/v1/*/latest`) | 100 req/min | Read-heavy, cheap |
| Snapshot list (`GET /api/v1/*/snapshots`) | 60 req/min | Paginated queries |
| Job list (`GET /api/v1/*/jobs`) | 60 req/min | Paginated queries |

These limits are per API key. Org-level aggregation and paid tiers are Phase 2.

**Middleware implementation:**

The middleware accepts an interface rather than a concrete `*redis.Client`, making it testable without Redis:

```go
// RateLimiter is defined by the consumer (middleware package).
// The Redis implementation satisfies it; tests use an in-memory stub.
type RateLimiter interface {
    Allow(ctx context.Context, key string, limit int, window time.Duration) (remaining int, resetAt time.Time, allowed bool, err error)
}

func RateLimitMiddleware(limiter RateLimiter, limits RateLimitConfig) func(http.Handler) http.Handler
```

Runs before the auth middleware so that rate-limited requests don't consume auth processing. The middleware extracts the identifier from the `Authorization` header (or falls back to client IP), determines the endpoint group from the route, and checks the limiter.

**Response headers on all API responses:**

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 73
X-RateLimit-Reset: 1710417600
```
