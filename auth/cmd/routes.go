package main

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	authmw "github.com/LegationPro/zagforge/auth/internal/middleware/auth"
	"github.com/LegationPro/zagforge/auth/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaplogger"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaprecoverer"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerRoutes(r *router.Router, h *handlers, rdb *redis.Client, tokenSvc *token.Service, log *zap.Logger) error {
	// Global middleware stack.
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zaplogger.Middleware(log))
	r.Use(zaprecoverer.Middleware(log))
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.ThrottleBacklog(100, 50, 5*time.Second))

	if err := registerPublicRoutes(r, h); err != nil {
		return err
	}
	if err := registerRateLimitedRoutes(r, h, rdb, log); err != nil {
		return err
	}
	return registerAuthenticatedRoutes(r, h, rdb, tokenSvc, log)
}

func registerPublicRoutes(r *router.Router, h *handlers) error {
	// Health — no auth, no rate limit.
	healthRoutes := r.Group()
	if err := healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: h.health.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: h.health.Readiness},
	}); err != nil {
		return fmt.Errorf("register health routes: %w", err)
	}

	// JWKS — public, no auth.
	jwksRoutes := r.Group()
	if err := jwksRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/.well-known/jwks.json", Handler: h.oauth.JWKS},
	}); err != nil {
		return fmt.Errorf("register jwks routes: %w", err)
	}

	// Public invite lookup — no auth (token is the secret).
	invitePublic := r.Group()
	if err := invitePublic.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/invites/{token}", Handler: h.invite.GetByToken},
	}); err != nil {
		return fmt.Errorf("register public invite routes: %w", err)
	}

	return nil
}

func registerRateLimitedRoutes(r *router.Router, h *handlers, rdb *redis.Client, log *zap.Logger) error {
	// OAuth — no auth, rate limited by IP.
	oauthRoutes := r.Group()
	oauthRoutes.Use(ratelimit.RateLimit(rdb, ratelimit.Config{
		MaxRequests: 30,
		Window:      1 * time.Minute,
	}, "oauth", log))
	if err := oauthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/oauth/{provider}/start", Handler: h.oauth.Start},
		{Method: router.GET, Path: "/auth/oauth/{provider}/callback", Handler: h.oauth.Callback},
	}); err != nil {
		return fmt.Errorf("register oauth routes: %w", err)
	}

	// Token refresh — rate limited by IP.
	refreshRoutes := r.Group()
	refreshRoutes.Use(ratelimit.RateLimit(rdb, ratelimit.Config{
		MaxRequests: 30,
		Window:      1 * time.Minute,
	}, "refresh", log))
	if err := refreshRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/token/refresh", Handler: h.session.Refresh},
		{Method: router.POST, Path: "/auth/logout", Handler: h.session.Logout},
	}); err != nil {
		return fmt.Errorf("register refresh routes: %w", err)
	}

	// MFA challenge — no auth, rate limited (uses mfa_challenge_token from OAuth callback).
	mfaPublic := r.Group()
	mfaPublic.Use(ratelimit.RateLimit(rdb, ratelimit.Config{
		MaxRequests: 10,
		Window:      1 * time.Minute,
	}, "mfa", log))
	if err := mfaPublic.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/mfa/totp/challenge", Handler: h.mfa.Challenge},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/verify", Handler: h.mfa.BackupCodeVerify},
	}); err != nil {
		return fmt.Errorf("register mfa challenge routes: %w", err)
	}

	return nil
}

func registerAuthenticatedRoutes(r *router.Router, h *handlers, rdb *redis.Client, tokenSvc *token.Service, log *zap.Logger) error {
	pubKey := tokenSvc.PublicKey()

	authed := r.Group()
	authed.Use(authmw.Auth(pubKey, tokenSvc.Issuer(), log))
	authed.Use(ratelimit.RateLimit(rdb, ratelimit.Config{
		MaxRequests: 60,
		Window:      1 * time.Minute,
	}, "auth", log))

	return authed.Create([]router.Subroute{
		// Sessions.
		{Method: router.POST, Path: "/auth/logout/all", Handler: h.session.LogoutAll},
		{Method: router.GET, Path: "/auth/sessions", Handler: h.session.ListSessions},
		{Method: router.DELETE, Path: "/auth/sessions/{sessionID}", Handler: h.session.RevokeSession},

		// MFA (authenticated — setup, verify, disable, regenerate codes).
		{Method: router.POST, Path: "/auth/mfa/totp/setup", Handler: h.mfa.Setup},
		{Method: router.POST, Path: "/auth/mfa/totp/verify", Handler: h.mfa.Verify},
		{Method: router.POST, Path: "/auth/mfa/totp/disable", Handler: h.mfa.Disable},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/generate", Handler: h.mfa.RegenerateBackupCodes},

		// User profile.
		{Method: router.GET, Path: "/auth/me", Handler: h.user.GetMe},
		{Method: router.PUT, Path: "/auth/me", Handler: h.user.UpdateMe},
		{Method: router.PUT, Path: "/auth/me/onboarding", Handler: h.user.UpdateOnboarding},
		{Method: router.GET, Path: "/auth/me/identities", Handler: h.user.ListIdentities},
		{Method: router.DELETE, Path: "/auth/me/identities/{provider}", Handler: h.user.UnlinkIdentity},

		// Organizations.
		{Method: router.POST, Path: "/auth/orgs", Handler: h.org.Create},
		{Method: router.GET, Path: "/auth/orgs", Handler: h.org.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}", Handler: h.org.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}", Handler: h.org.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}", Handler: h.org.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/members", Handler: h.org.ListMembers},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: h.org.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: h.org.RemoveMember},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/transfer", Handler: h.org.TransferOwnership},

		// Invites (org-scoped, requires auth).
		{Method: router.POST, Path: "/auth/orgs/{orgID}/invites", Handler: h.invite.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/invites", Handler: h.invite.ListOrgInvites},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/invites/{inviteID}", Handler: h.invite.Revoke},
		{Method: router.POST, Path: "/auth/invites/accept", Handler: h.invite.Accept},

		// Teams.
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams", Handler: h.team.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams", Handler: h.team.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: h.team.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: h.team.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: h.team.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: h.team.ListMembers},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: h.team.AddMember},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: h.team.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: h.team.RemoveMember},

		// Audit logs + metrics.
		{Method: router.GET, Path: "/auth/orgs/{orgID}/audit-logs", Handler: h.audit.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/logins", Handler: h.audit.LoginMetrics},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/failed-logins", Handler: h.audit.FailedLoginMetrics},

		// Webhooks.
		{Method: router.POST, Path: "/auth/orgs/{orgID}/webhooks", Handler: h.webhook.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/webhooks", Handler: h.webhook.List},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/webhooks/{whID}", Handler: h.webhook.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/webhooks/{whID}", Handler: h.webhook.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/webhooks/{whID}/deliveries", Handler: h.webhook.ListDeliveries},

		// Admin (platform admin only).
		{Method: router.GET, Path: "/auth/admin/users", Handler: h.admin.ListUsers},
		{Method: router.GET, Path: "/auth/admin/users/{userID}", Handler: h.admin.GetUser},
		{Method: router.PUT, Path: "/auth/admin/users/{userID}", Handler: h.admin.UpdateUser},
		{Method: router.GET, Path: "/auth/admin/orgs", Handler: h.admin.ListOrgs},
		{Method: router.PUT, Path: "/auth/admin/orgs/{orgID}", Handler: h.admin.UpdateOrgPlan},
	})
}
