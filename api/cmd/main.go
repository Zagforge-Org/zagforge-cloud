package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	"github.com/LegationPro/zagforge/api/internal/config"
	"github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/engine"
	aikeyshandler "github.com/LegationPro/zagforge/api/internal/handler/aikeys"
	apihandler "github.com/LegationPro/zagforge/api/internal/handler/api"
	"github.com/LegationPro/zagforge/api/internal/handler/callback"
	contexttokenshandler "github.com/LegationPro/zagforge/api/internal/handler/contexttokens"
	contexturlhandler "github.com/LegationPro/zagforge/api/internal/handler/contexturl"
	"github.com/LegationPro/zagforge/api/internal/handler/githubauth"
	"github.com/LegationPro/zagforge/api/internal/handler/health"
	queryhandler "github.com/LegationPro/zagforge/api/internal/handler/query"
	uploadhandler "github.com/LegationPro/zagforge/api/internal/handler/upload"
	"github.com/LegationPro/zagforge/api/internal/handler/watchdog"
	"github.com/LegationPro/zagforge/api/internal/handler/webhook"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/middleware/bodylimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/contenttype"
	corsmw "github.com/LegationPro/zagforge/api/internal/middleware/cors"
	jobtokenmw "github.com/LegationPro/zagforge/api/internal/middleware/jobtoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/watchdogauth"
	"github.com/LegationPro/zagforge/api/internal/service"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/shared/go/jobtoken"
	"github.com/LegationPro/zagforge/shared/go/logger"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	"github.com/LegationPro/zagforge/shared/go/router"
	storagepkg "github.com/LegationPro/zagforge/shared/go/storage"
)

func run() error {
	c, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(os.Getenv("APP_ENV"))
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	pool, err := db.Connect(context.Background(), c.DB.URL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer pool.Close()

	database := db.New(pool)

	// Redis for rate limiting.
	redisOpts, err := redis.ParseURL(c.Redis.URL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Warn("failed to close redis", zap.Error(err))
		}
	}()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}

	client, err := githubprovider.NewAPIClient(c.App.GithubAppID, []byte(c.App.GithubAppPrivateKey), c.App.GithubAppWebhookSecret)
	if err != nil {
		return fmt.Errorf("create API client: %w", err)
	}

	ch, err := githubprovider.NewClientHandler(client, log)
	if err != nil {
		return fmt.Errorf("create client handler: %w", err)
	}

	clerk.SetKey(c.App.ClerkSecretKey)

	signer := jobtoken.NewSigner([]byte(c.App.HMACSigningKey), 30*time.Minute)
	if c.App.HMACSigningKeyPrev != "" {
		signer = signer.WithPreviousKey([]byte(c.App.HMACSigningKeyPrev))
		log.Info("HMAC key rotation active: accepting both current and previous signing keys")
	}

	// Cloud Tasks enqueuer (or noop for local dev).
	var enqueuer engine.TaskEnqueuer
	if c.CloudTasks.Enabled() {
		ct, err := engine.NewCloudTasksEnqueuer(context.Background(), engine.CloudTasksConfig{
			Project:        c.CloudTasks.Project,
			Location:       c.CloudTasks.Location,
			Queue:          c.CloudTasks.Queue,
			WorkerURL:      c.CloudTasks.WorkerURL,
			ServiceAccount: c.CloudTasks.ServiceAccount,
		})
		if err != nil {
			return fmt.Errorf("create cloud tasks enqueuer: %w", err)
		}
		defer func() { _ = ct.Close() }()
		enqueuer = ct
		log.Info("cloud tasks enqueuer enabled",
			zap.String("queue", c.CloudTasks.Queue),
			zap.String("worker_url", c.CloudTasks.WorkerURL),
		)
	} else {
		enqueuer = engine.NewNoopEnqueuer(log)
		log.Info("cloud tasks not configured, using noop enqueuer (poller mode)")
	}

	// GCS client for snapshot storage.
	gcsClient, err := storagepkg.NewClient(context.Background(), storagepkg.Config{
		Bucket:   c.GCS.Bucket,
		Endpoint: c.GCS.Endpoint,
	}, log)
	if err != nil {
		return fmt.Errorf("create gcs client: %w", err)
	}

	// Encryption service for AI provider keys.
	encKeyBytes, err := base64.StdEncoding.DecodeString(c.App.EncryptionKeyBase64)
	if err != nil {
		return fmt.Errorf("decode encryption key: %w", err)
	}
	encSvc, err := encryption.New(encKeyBytes)
	if err != nil {
		return fmt.Errorf("init encryption: %w", err)
	}

	ctxCache := contextcache.NewRedis(rdb)

	svc := service.NewJobService(database, log, enqueuer, signer)
	wh := webhook.NewHandler(ch, svc, log)
	healthH := health.NewHandler(pool)
	apiH := apihandler.NewHandler(database, log)
	callbackH := callback.NewHandler(database, ch, log)
	watchdogH := watchdog.NewHandler(database, log)
	githubAuthH := githubauth.NewHandler(database, c.App.GithubAppSlug, log)

	// Phase 5 handlers.
	uploadH := uploadhandler.NewHandler(database, gcsClient, log)
	contextURLH := contexturlhandler.NewHandler(database, ctxCache, ch, gcsClient, log)
	ctxTokensH := contexttokenshandler.NewHandler(database, log)
	aiKeysH := aikeyshandler.NewHandler(database, encSvc, log)
	queryH := queryhandler.NewHandler(database, ctxCache, ch, gcsClient, encSvc, log)

	r := router.New()

	// Health — no auth, no rate limit.
	healthRoutes := r.Group()
	if err := healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: healthH.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: healthH.Readiness},
	}); err != nil {
		return fmt.Errorf("register health routes: %w", err)
	}

	// GitHub App OAuth — no auth, rate limited by IP to prevent abuse.
	authRoutes := r.Group()
	authRoutes.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: 30,
		Window:      1 * time.Minute,
	}, "oauth", log))
	if err := authRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/github/install", Handler: githubAuthH.Install},
		{Method: router.GET, Path: "/auth/github/callback", Handler: githubAuthH.Callback},
	}); err != nil {
		return fmt.Errorf("register github auth routes: %w", err)
	}

	// Webhooks — body limit + Content-Type + rate limited by IP, higher burst (GitHub sends bursts).
	internal := r.Group()
	internal.Use(bodylimit.Limit(1 << 20)) // 1MB max
	internal.Use(contenttype.RequireJSON())
	internal.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: 120,
		Window:      1 * time.Minute,
	}, "webhook", log))
	if err := internal.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/webhooks/github", Handler: wh.ServeHTTP},
	}); err != nil {
		return fmt.Errorf("register internal routes: %w", err)
	}

	// Job callbacks — body limit + Content-Type + signed job token auth.
	callbacks := r.Group()
	callbacks.Use(bodylimit.Limit(1 << 20)) // 1MB max
	callbacks.Use(contenttype.RequireJSON())
	callbacks.Use(jobtokenmw.Auth(signer, log))
	if err := callbacks.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/jobs/start", Handler: callbackH.Start},
		{Method: router.POST, Path: "/internal/jobs/complete", Handler: callbackH.Complete},
	}); err != nil {
		return fmt.Errorf("register callback routes: %w", err)
	}

	// Watchdog — shared secret auth (Cloud Scheduler in production uses OIDC).
	watchdogRoutes := r.Group()
	watchdogRoutes.Use(watchdogauth.SharedSecret(c.App.WatchdogSecret))
	if err := watchdogRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/watchdog/timeout", Handler: watchdogH.Timeout},
	}); err != nil {
		return fmt.Errorf("register watchdog routes: %w", err)
	}

	// API v1 — restricted CORS + auth + org scoping + rate limit.
	v1 := r.Group()
	v1.Use(corsmw.Cors(c.CORS.AllowedOrigins))
	v1.Use(auth.Auth(log))
	v1.Use(auth.OrgScope(database.Queries, log))
	v1.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: 60,
		Window:      1 * time.Minute,
	}, "api", log))
	if err := v1.Create([]router.Subroute{
		{Method: router.GET, Path: "/api/v1/repos/{repoID}", Handler: apiH.GetRepo},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs", Handler: apiH.ListJobs},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs/{jobID}", Handler: apiH.GetJob},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots", Handler: apiH.ListSnapshots},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots/latest", Handler: apiH.GetLatestSnapshot},
		{Method: router.GET, Path: "/api/v1/snapshots/{snapshotID}", Handler: apiH.GetSnapshot},
	}); err != nil {
		return fmt.Errorf("register api routes: %w", err)
	}

	// Context URL — no auth (token is the secret), restricted CORS for dashboard.
	contextRoutes := r.Group()
	contextRoutes.Use(corsmw.Cors(c.CORS.AllowedOrigins))
	if err := contextRoutes.Create([]router.Subroute{
		{Method: router.HEAD, Path: "/v1/context/{token}", Handler: contextURLH.Head},
		{Method: router.GET, Path: "/v1/context/{token}", Handler: contextURLH.Get},
	}); err != nil {
		return fmt.Errorf("register context url routes: %w", err)
	}

	// CLI upload — body limit + CLI token auth + rate limited.
	uploadRoutes := r.Group()
	uploadRoutes.Use(bodylimit.Limit(10 << 20)) // 10MB max
	uploadRoutes.Use(contenttype.RequireJSON())
	uploadRoutes.Use(clitoken.Auth(c.App.CLIAPIKey))
	uploadRoutes.Use(ratelimit.RateLimit(rdb, ratelimit.RateLimitConfig{
		MaxRequests: 60,
		Window:      1 * time.Minute,
	}, "upload", log))
	if err := uploadRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/api/v1/upload", Handler: uploadH.Upload},
	}); err != nil {
		return fmt.Errorf("register upload routes: %w", err)
	}

	// Context tokens + AI keys + Query — Clerk auth + rate limited.
	if err := v1.Create([]router.Subroute{
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: ctxTokensH.List},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: ctxTokensH.Create},
		{Method: router.DELETE, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}", Handler: ctxTokensH.Delete},
		{Method: router.GET, Path: "/api/v1/orgs/settings/ai-keys", Handler: aiKeysH.List},
		{Method: router.PUT, Path: "/api/v1/orgs/settings/ai-keys", Handler: aiKeysH.Upsert},
		{Method: router.DELETE, Path: "/api/v1/orgs/settings/ai-keys/{provider}", Handler: aiKeysH.Delete},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/query", Handler: queryH.Query},
	}); err != nil {
		return fmt.Errorf("register phase 5 routes: %w", err)
	}

	srv := &http.Server{
		Addr:    ":" + c.Server.Port,
		Handler: r.Handler(),
	}

	go func() {
		log.Info("server listening", zap.String("port", c.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Info("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("server stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
