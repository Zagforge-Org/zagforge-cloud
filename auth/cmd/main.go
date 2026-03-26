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

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/config"
	"github.com/LegationPro/zagforge/auth/internal/db"
	audithandler "github.com/LegationPro/zagforge/auth/internal/handler/audit"
	"github.com/LegationPro/zagforge/auth/internal/handler/health"
	invitehandler "github.com/LegationPro/zagforge/auth/internal/handler/invite"
	mfahandler "github.com/LegationPro/zagforge/auth/internal/handler/mfa"
	oauthhandler "github.com/LegationPro/zagforge/auth/internal/handler/oauth"
	orghandler "github.com/LegationPro/zagforge/auth/internal/handler/org"
	sessionhandler "github.com/LegationPro/zagforge/auth/internal/handler/session"
	teamhandler "github.com/LegationPro/zagforge/auth/internal/handler/team"
	userhandler "github.com/LegationPro/zagforge/auth/internal/handler/user"
	authmw "github.com/LegationPro/zagforge/auth/internal/middleware/auth"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/encryption"
	oauthsvc "github.com/LegationPro/zagforge/auth/internal/service/oauth"
	oauthgithub "github.com/LegationPro/zagforge/auth/internal/service/oauth/github"
	oauthgoogle "github.com/LegationPro/zagforge/auth/internal/service/oauth/google"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	"github.com/LegationPro/zagforge/shared/go/logger"
	"github.com/LegationPro/zagforge/shared/go/router"
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

	// Database.
	pool, err := db.Connect(context.Background(), c.DB.URL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer pool.Close()

	database := db.New(pool)

	// Redis.
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

	// Token service (JWT).
	tokenSvc, err := token.New(
		c.App.JWTPrivateKeyBase64,
		c.App.JWTPublicKeyBase64,
		c.App.JWTIssuer,
		c.App.JWTAccessTokenTTL,
		c.App.JWTRefreshTokenTTL,
	)
	if err != nil {
		return fmt.Errorf("init token service: %w", err)
	}

	// Encryption service.
	encKeyBytes, err := base64.StdEncoding.DecodeString(c.App.EncryptionKeyBase64)
	if err != nil {
		return fmt.Errorf("decode encryption key: %w", err)
	}
	encSvc, err := encryption.New(encKeyBytes)
	if err != nil {
		return fmt.Errorf("init encryption: %w", err)
	}

	// OAuth providers.
	providers := map[string]oauthsvc.Provider{
		"github": oauthgithub.New(c.App.GithubOAuthClientID, c.App.GithubOAuthClientSecret, c.App.OAuthCallbackBaseURL),
		"google": oauthgoogle.New(c.App.GoogleOAuthClientID, c.App.GoogleOAuthClientSecret, c.App.OAuthCallbackBaseURL),
	}

	// Services.
	sessionSvc := sessionsvc.New(database.Queries, c.App.SessionMaxAge)
	auditSvc := audit.New(database.Queries)

	// Handlers.
	healthH := health.NewHandler(pool)
	oauthH := oauthhandler.NewHandler(database, providers, tokenSvc, sessionSvc, encSvc, auditSvc, log, c.App.FrontendURL, c.App.JWKSKeyID)
	sessionH := sessionhandler.NewHandler(database, tokenSvc, sessionSvc, auditSvc, log)
	userH := userhandler.NewHandler(database, log)
	orgH := orghandler.NewHandler(database, auditSvc, log)
	inviteH := invitehandler.NewHandler(database, auditSvc, log)
	mfaH := mfahandler.NewHandler(database, tokenSvc, sessionSvc, encSvc, auditSvc, log)
	teamH := teamhandler.NewHandler(database, auditSvc, log)
	auditH := audithandler.NewHandler(database, log)

	// Parse public key for auth middleware.
	pubKey := tokenSvc.PublicKey()

	r := router.New()

	// Health — no auth, no rate limit.
	healthRoutes := r.Group()
	if err := healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: healthH.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: healthH.Readiness},
	}); err != nil {
		return fmt.Errorf("register health routes: %w", err)
	}

	// JWKS — public, no auth.
	jwksRoutes := r.Group()
	if err := jwksRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/.well-known/jwks.json", Handler: oauthH.JWKS},
	}); err != nil {
		return fmt.Errorf("register jwks routes: %w", err)
	}

	// OAuth — no auth, rate limited.
	oauthRoutes := r.Group()
	// TODO: add rate limiting once ratelimit middleware is ported
	if err := oauthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/oauth/{provider}/start", Handler: oauthH.Start},
		{Method: router.GET, Path: "/auth/oauth/{provider}/callback", Handler: oauthH.Callback},
	}); err != nil {
		return fmt.Errorf("register oauth routes: %w", err)
	}

	// Token refresh — no auth (uses refresh token cookie/body).
	refreshRoutes := r.Group()
	if err := refreshRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/token/refresh", Handler: sessionH.Refresh},
		{Method: router.POST, Path: "/auth/logout", Handler: sessionH.Logout},
	}); err != nil {
		return fmt.Errorf("register refresh routes: %w", err)
	}

	// MFA challenge — no auth (uses mfa_challenge_token from OAuth callback).
	mfaPublic := r.Group()
	if err := mfaPublic.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/mfa/totp/challenge", Handler: mfaH.Challenge},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/verify", Handler: mfaH.BackupCodeVerify},
	}); err != nil {
		return fmt.Errorf("register mfa challenge routes: %w", err)
	}

	// Authenticated routes.
	authed := r.Group()
	authed.Use(authmw.Auth(pubKey, tokenSvc.Issuer(), log))
	if err := authed.Create([]router.Subroute{
		// Sessions.
		{Method: router.POST, Path: "/auth/logout/all", Handler: sessionH.LogoutAll},
		{Method: router.GET, Path: "/auth/sessions", Handler: sessionH.ListSessions},
		{Method: router.DELETE, Path: "/auth/sessions/{sessionID}", Handler: sessionH.RevokeSession},

		// MFA (authenticated — setup, verify, disable, regenerate codes).
		{Method: router.POST, Path: "/auth/mfa/totp/setup", Handler: mfaH.Setup},
		{Method: router.POST, Path: "/auth/mfa/totp/verify", Handler: mfaH.Verify},
		{Method: router.POST, Path: "/auth/mfa/totp/disable", Handler: mfaH.Disable},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/generate", Handler: mfaH.RegenerateBackupCodes},

		// User profile.
		{Method: router.GET, Path: "/auth/me", Handler: userH.GetMe},
		{Method: router.PUT, Path: "/auth/me", Handler: userH.UpdateMe},
		{Method: router.PUT, Path: "/auth/me/onboarding", Handler: userH.UpdateOnboarding},
		{Method: router.GET, Path: "/auth/me/identities", Handler: userH.ListIdentities},
		{Method: router.DELETE, Path: "/auth/me/identities/{provider}", Handler: userH.UnlinkIdentity},

		// Organizations.
		{Method: router.POST, Path: "/auth/orgs", Handler: orgH.Create},
		{Method: router.GET, Path: "/auth/orgs", Handler: orgH.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}", Handler: orgH.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}", Handler: orgH.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}", Handler: orgH.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/members", Handler: orgH.ListMembers},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: orgH.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: orgH.RemoveMember},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/transfer", Handler: orgH.TransferOwnership},

		// Invites (org-scoped, requires auth).
		{Method: router.POST, Path: "/auth/orgs/{orgID}/invites", Handler: inviteH.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/invites", Handler: inviteH.ListOrgInvites},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/invites/{inviteID}", Handler: inviteH.Revoke},
		{Method: router.POST, Path: "/auth/invites/accept", Handler: inviteH.Accept},

		// Teams.
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams", Handler: teamH.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams", Handler: teamH.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: teamH.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: teamH.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: teamH.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: teamH.ListMembers},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: teamH.AddMember},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: teamH.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: teamH.RemoveMember},

		// Audit logs + metrics.
		{Method: router.GET, Path: "/auth/orgs/{orgID}/audit-logs", Handler: auditH.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/logins", Handler: auditH.LoginMetrics},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/failed-logins", Handler: auditH.FailedLoginMetrics},
	}); err != nil {
		return fmt.Errorf("register authenticated routes: %w", err)
	}

	// Public invite lookup — no auth (token is the secret).
	invitePublic := r.Group()
	if err := invitePublic.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/invites/{token}", Handler: inviteH.GetByToken},
	}); err != nil {
		return fmt.Errorf("register public invite routes: %w", err)
	}

	srv := &http.Server{
		Addr:    ":" + c.Server.Port,
		Handler: r.Handler(),
	}

	go func() {
		log.Info("auth server listening", zap.String("port", c.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()

	log.Info("shutting down auth server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("auth server stopped")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
