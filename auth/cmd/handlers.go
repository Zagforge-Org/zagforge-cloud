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
)

type handlers struct {
	health  *health.Handler
	oauth   *oauthhandler.Handler
	session *sessionhandler.Handler
	user    *userhandler.Handler
	org     *orghandler.Handler
	invite  *invitehandler.Handler
	mfa     *mfahandler.Handler
	team    *teamhandler.Handler
	audit   *audithandler.Handler
	webhook *webhookhandler.Handler
	admin   *adminhandler.Handler
}

func initHandlers(d *deps, c *config.Config, log *zap.Logger) *handlers {
	return &handlers{
		health:  health.NewHandler(d.pool),
		oauth:   oauthhandler.NewHandler(d.database, d.providers, d.tokenSvc, d.sessionSvc, d.encSvc, d.auditSvc, log, c.App.FrontendURL, c.App.JWKSKeyID),
		session: sessionhandler.NewHandler(d.database, d.tokenSvc, d.sessionSvc, d.auditSvc, log),
		user:    userhandler.NewHandler(d.database, log),
		org:     orghandler.NewHandler(d.database, d.auditSvc, log),
		invite:  invitehandler.NewHandler(d.database, d.auditSvc, log),
		mfa:     mfahandler.NewHandler(d.database, d.tokenSvc, d.sessionSvc, d.encSvc, d.auditSvc, log),
		team:    teamhandler.NewHandler(d.database, d.auditSvc, log),
		audit:   audithandler.NewHandler(d.database, log),
		webhook: webhookhandler.NewHandler(d.database, log),
		admin:   adminhandler.NewHandler(d.database, log),
	}
}
