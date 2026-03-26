package main

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/config"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/middleware/bodylimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/contenttype"
	corsmw "github.com/LegationPro/zagforge/api/internal/middleware/cors"
	jobtokenmw "github.com/LegationPro/zagforge/api/internal/middleware/jobtoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/watchdogauth"
	"github.com/LegationPro/zagforge/shared/go/jobtoken"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaplogger"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaprecoverer"
	"github.com/LegationPro/zagforge/shared/go/router"
)

const (
	bodyLimit1MB  = 1 << 20
	bodyLimit10MB = 10 << 20

	rateLimitOAuth   = 30
	rateLimitWebhook = 120
	rateLimitAPI     = 60
	rateLimitUpload  = 60

	rateLimitWindow = 1 * time.Minute
)

func registerRoutes(r *router.Router, h *handlers, d *deps, c *config.Config, log *zap.Logger) error {
	// Global middleware stack.
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zaplogger.Middleware(log))
	r.Use(zaprecoverer.Middleware(log))
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.ThrottleBacklog(100, 50, 5*time.Second))

	if err := registerHealthRoutes(r, h); err != nil {
		return err
	}
	if err := registerGitHubAuthRoutes(r, h, d.rdb, log); err != nil {
		return err
	}
	if err := registerInternalRoutes(r, h, d.rdb, d.signer, log); err != nil {
		return err
	}
	if err := registerAPIv1Routes(r, h, d, c, log); err != nil {
		return err
	}
	return registerContextAndUploadRoutes(r, h, d, c, log)
}

func registerHealthRoutes(r *router.Router, h *handlers) error {
	healthRoutes := r.Group()
	return healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: h.health.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: h.health.Readiness},
	})
}

func registerGitHubAuthRoutes(r *router.Router, h *handlers, rdb *redis.Client, log *zap.Logger) error {
	authRoutes := r.Group()
	authRoutes.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitOAuth,
		Window:      rateLimitWindow,
	}, "oauth", log))
	return authRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/github/install", Handler: h.githubAuth.Install},
		{Method: router.GET, Path: "/auth/github/callback", Handler: h.githubAuth.Callback},
	})
}

func registerInternalRoutes(r *router.Router, h *handlers, rdb *redis.Client, signer *jobtoken.Signer, log *zap.Logger) error {
	// Webhooks — body limit + Content-Type + rate limited.
	internal := r.Group()
	internal.Use(bodylimit.Limit(bodyLimit1MB))
	internal.Use(contenttype.RequireJSON())
	internal.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitWebhook,
		Window:      rateLimitWindow,
	}, "webhook", log))
	if err := internal.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/webhooks/github", Handler: h.webhook.ServeHTTP},
	}); err != nil {
		return fmt.Errorf("register webhook routes: %w", err)
	}

	// Job callbacks — body limit + Content-Type + signed job token auth.
	callbacks := r.Group()
	callbacks.Use(bodylimit.Limit(bodyLimit1MB))
	callbacks.Use(contenttype.RequireJSON())
	callbacks.Use(jobtokenmw.Auth(signer, log))
	if err := callbacks.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/jobs/start", Handler: h.callback.Start},
		{Method: router.POST, Path: "/internal/jobs/complete", Handler: h.callback.Complete},
	}); err != nil {
		return fmt.Errorf("register callback routes: %w", err)
	}

	return nil
}

func registerAPIv1Routes(r *router.Router, h *handlers, d *deps, c *config.Config, log *zap.Logger) error {
	// Watchdog — shared secret auth.
	watchdogRoutes := r.Group()
	watchdogRoutes.Use(watchdogauth.SharedSecret(c.App.WatchdogSecret))
	if err := watchdogRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/watchdog/timeout", Handler: h.watchdog.Timeout},
	}); err != nil {
		return fmt.Errorf("register watchdog routes: %w", err)
	}

	// API v1 — restricted CORS + auth + scope resolution + rate limit.
	v1 := r.Group()
	v1.Use(corsmw.Cors(c.CORS.AllowedOrigins))
	v1.Use(auth.Auth(d.jwtPubKey, c.App.JWTIssuer, log))
	v1.Use(auth.Scope(log))
	v1.Use(ratelimit.RateLimit(d.rdb, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitAPI,
		Window:      rateLimitWindow,
	}, "api", log))
	return v1.Create([]router.Subroute{
		// Repos & jobs.
		{Method: router.GET, Path: "/api/v1/repos/{repoID}", Handler: h.api.GetRepo},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs", Handler: h.api.ListJobs},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs/{jobID}", Handler: h.api.GetJob},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots", Handler: h.api.ListSnapshots},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots/latest", Handler: h.api.GetLatestSnapshot},
		{Method: router.GET, Path: "/api/v1/snapshots/{snapshotID}", Handler: h.api.GetSnapshot},

		// Context tokens + AI keys + Query.
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: h.ctxTokens.List},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: h.ctxTokens.Create},
		{Method: router.DELETE, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}", Handler: h.ctxTokens.Delete},
		{Method: router.GET, Path: "/api/v1/orgs/settings/ai-keys", Handler: h.aiKeys.List},
		{Method: router.PUT, Path: "/api/v1/orgs/settings/ai-keys", Handler: h.aiKeys.Upsert},
		{Method: router.DELETE, Path: "/api/v1/orgs/settings/ai-keys/{provider}", Handler: h.aiKeys.Delete},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/query", Handler: h.query.Query},

		// Account management.
		{Method: router.GET, Path: "/api/v1/account", Handler: h.account.GetProfile},
		{Method: router.PATCH, Path: "/api/v1/account", Handler: h.account.UpdateProfile},
		{Method: router.DELETE, Path: "/api/v1/account", Handler: h.account.DeleteAccount},
		{Method: router.GET, Path: "/api/v1/account/sessions", Handler: h.account.ListSessions},
		{Method: router.DELETE, Path: "/api/v1/account/sessions/{id}", Handler: h.account.RevokeSession},

		// Organization management.
		{Method: router.POST, Path: "/api/v1/orgs", Handler: h.org.CreateOrg},
		{Method: router.GET, Path: "/api/v1/orgs", Handler: h.org.ListOrgs},
		{Method: router.PATCH, Path: "/api/v1/orgs/{orgID}", Handler: h.org.UpdateOrg},
		{Method: router.DELETE, Path: "/api/v1/orgs/{orgID}", Handler: h.org.DeleteOrg},
		{Method: router.GET, Path: "/api/v1/orgs/{orgID}/members", Handler: h.org.ListMembers},
		{Method: router.POST, Path: "/api/v1/orgs/{orgID}/members", Handler: h.org.InviteMember},
		{Method: router.PATCH, Path: "/api/v1/orgs/{orgID}/members/{userID}", Handler: h.org.UpdateMemberRole},
		{Method: router.DELETE, Path: "/api/v1/orgs/{orgID}/members/{userID}", Handler: h.org.RemoveMember},
		{Method: router.GET, Path: "/api/v1/orgs/{orgID}/audit-log", Handler: h.org.ListAuditLog},
	})
}

func registerContextAndUploadRoutes(r *router.Router, h *handlers, d *deps, c *config.Config, log *zap.Logger) error {
	// Context URL — no auth (token is the secret), restricted CORS for dashboard.
	contextRoutes := r.Group()
	contextRoutes.Use(corsmw.Cors(c.CORS.AllowedOrigins))
	if err := contextRoutes.Create([]router.Subroute{
		{Method: router.HEAD, Path: "/v1/context/{token}", Handler: h.contextURL.Head},
		{Method: router.GET, Path: "/v1/context/{token}", Handler: h.contextURL.Get},
	}); err != nil {
		return fmt.Errorf("register context url routes: %w", err)
	}

	// CLI upload — body limit + CLI token auth + rate limited.
	uploadRoutes := r.Group()
	uploadRoutes.Use(bodylimit.Limit(bodyLimit10MB))
	uploadRoutes.Use(contenttype.RequireJSON())
	uploadRoutes.Use(clitoken.Auth(c.App.CLIAPIKey))
	uploadRoutes.Use(ratelimit.RateLimit(d.rdb, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitUpload,
		Window:      rateLimitWindow,
	}, "upload", log))
	return uploadRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/api/v1/upload", Handler: h.upload.Upload},
	})
}
