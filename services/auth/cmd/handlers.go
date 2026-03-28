package main

import (
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/config"
	adminhandler "github.com/LegationPro/zagforge/auth/internal/handler/admin"
	audithandler "github.com/LegationPro/zagforge/auth/internal/handler/audit"
	"github.com/LegationPro/zagforge/auth/internal/handler/health"
	invitehandler "github.com/LegationPro/zagforge/auth/internal/handler/invite"
	mfahandler "github.com/LegationPro/zagforge/auth/internal/handler/mfa"
	oauthhandler "github.com/LegationPro/zagforge/auth/internal/handler/oauth"
	orghandler "github.com/LegationPro/zagforge/auth/internal/handler/org"
	sessionhandler "github.com/LegationPro/zagforge/auth/internal/handler/session"
	teamhandler "github.com/LegationPro/zagforge/auth/internal/handler/team"
	userhandler "github.com/LegationPro/zagforge/auth/internal/handler/user"
	webhookhandler "github.com/LegationPro/zagforge/auth/internal/handler/webhook"
	"github.com/LegationPro/zagforge/auth/internal/routes"
)

func newRouteDeps(d *deps, c *config.Config, log *zap.Logger) *routes.Deps {
	return &routes.Deps{
		Health:  health.NewHandler(d.pool),
		OAuth:   oauthhandler.NewHandler(d.database, d.providers, d.tokenSvc, d.sessionSvc, d.encSvc, d.auditSvc, log, c.App.FrontendURL, c.App.JWKSKeyID),
		Session: sessionhandler.NewHandler(d.database, d.tokenSvc, d.sessionSvc, d.auditSvc, log),
		User:    userhandler.NewHandler(d.database, log),
		Org:     orghandler.NewHandler(d.database, d.auditSvc, log),
		Invite:  invitehandler.NewHandler(d.database, d.auditSvc, log),
		MFA:     mfahandler.NewHandler(d.database, d.tokenSvc, d.sessionSvc, d.encSvc, d.auditSvc, log),
		Team:    teamhandler.NewHandler(d.database, d.auditSvc, log),
		Audit:   audithandler.NewHandler(d.database, log),
		Webhook: webhookhandler.NewHandler(d.database, log),
		Admin:   adminhandler.NewHandler(d.database, log),

		RDB:         d.rdb,
		PubKey:      d.tokenSvc.PublicKey(),
		JWTIssuer:   d.tokenSvc.Issuer(),
		CORSOrigins: c.CORS.AllowedOrigins,
		Log:         log,
	}
}
