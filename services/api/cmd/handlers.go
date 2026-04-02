package main

import (
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/config"
	accounthandler "github.com/LegationPro/zagforge/api/internal/handler/account"
	aikeyshandler "github.com/LegationPro/zagforge/api/internal/handler/aikeys"
	apihandler "github.com/LegationPro/zagforge/api/internal/handler/api"
	"github.com/LegationPro/zagforge/api/internal/handler/callback"
	clikeyshandler "github.com/LegationPro/zagforge/api/internal/handler/clikeys"
	contexttokenshandler "github.com/LegationPro/zagforge/api/internal/handler/contexttokens"
	contexturlhandler "github.com/LegationPro/zagforge/api/internal/handler/contexturl"
	"github.com/LegationPro/zagforge/api/internal/handler/githubauth"
	"github.com/LegationPro/zagforge/api/internal/handler/health"
	orghandler "github.com/LegationPro/zagforge/api/internal/handler/org"
	queryhandler "github.com/LegationPro/zagforge/api/internal/handler/query"
	uploadhandler "github.com/LegationPro/zagforge/api/internal/handler/upload"
	"github.com/LegationPro/zagforge/api/internal/handler/watchdog"
	"github.com/LegationPro/zagforge/api/internal/handler/webhook"
	"github.com/LegationPro/zagforge/api/internal/routes"
	"github.com/LegationPro/zagforge/api/internal/service"
)

func newRouteDeps(d *deps, c *config.Config, log *zap.Logger) *routes.Deps {
	svc := service.NewJobService(d.database, log, d.enqueuer, d.signer)

	return &routes.Deps{
		Health:     health.NewHandler(d.pool),
		Webhook:    webhook.NewHandler(d.ch, svc, log),
		API:        apihandler.NewHandler(d.database, log),
		Callback:   callback.NewHandler(d.database, d.ch, log),
		Watchdog:   watchdog.NewHandler(d.database, log),
		GithubAuth: githubauth.NewHandler(d.database, c.App.GithubAppSlug, log),
		Upload:     uploadhandler.NewHandler(d.database, d.gcsClient, log),
		ContextURL: contexturlhandler.NewHandler(d.database, d.ctxCache, d.ch, d.gcsClient, log, d.jwtPubKey, c.App.JWTIssuer),
		CtxTokens:  contexttokenshandler.NewHandler(d.database, log),
		AIKeys:     aikeyshandler.NewHandler(d.database, d.encSvc, log),
		CLIKeys:    clikeyshandler.NewHandler(d.database, log),
		Query:      queryhandler.NewHandler(d.database, d.ctxCache, d.ch, d.gcsClient, d.encSvc, log),
		Account:    accounthandler.NewHandler(d.database, log),
		Org:        orghandler.NewHandler(d.database, log),

		Queries:        d.database.Queries,
		RDB:            d.rdb,
		JWTPubKey:      d.jwtPubKey,
		JWTIssuer:      c.App.JWTIssuer,
		Signer:         d.signer,
		CORSOrigins:    c.CORS.AllowedOrigins,
		WatchdogSecret: c.App.WatchdogSecret,
		CLIAPIKey:      c.App.CLIAPIKey,
		Log:            log,
	}
}
