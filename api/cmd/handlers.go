package main

import (
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/config"
	accounthandler "github.com/LegationPro/zagforge/api/internal/handler/account"
	aikeyshandler "github.com/LegationPro/zagforge/api/internal/handler/aikeys"
	apihandler "github.com/LegationPro/zagforge/api/internal/handler/api"
	"github.com/LegationPro/zagforge/api/internal/handler/callback"
	contexttokenshandler "github.com/LegationPro/zagforge/api/internal/handler/contexttokens"
	contexturlhandler "github.com/LegationPro/zagforge/api/internal/handler/contexturl"
	"github.com/LegationPro/zagforge/api/internal/handler/githubauth"
	"github.com/LegationPro/zagforge/api/internal/handler/health"
	orghandler "github.com/LegationPro/zagforge/api/internal/handler/org"
	queryhandler "github.com/LegationPro/zagforge/api/internal/handler/query"
	uploadhandler "github.com/LegationPro/zagforge/api/internal/handler/upload"
	"github.com/LegationPro/zagforge/api/internal/handler/watchdog"
	"github.com/LegationPro/zagforge/api/internal/handler/webhook"
	"github.com/LegationPro/zagforge/api/internal/service"
)

type handlers struct {
	health     *health.Handler
	webhook    *webhook.Handler
	api        *apihandler.Handler
	callback   *callback.Handler
	watchdog   *watchdog.Handler
	githubAuth *githubauth.Handler
	upload     *uploadhandler.Handler
	contextURL *contexturlhandler.Handler
	ctxTokens  *contexttokenshandler.Handler
	aiKeys     *aikeyshandler.Handler
	query      *queryhandler.Handler
	account    *accounthandler.Handler
	org        *orghandler.Handler
}

func initHandlers(d *deps, c *config.Config, log *zap.Logger) *handlers {
	svc := service.NewJobService(d.database, log, d.enqueuer, d.signer)

	return &handlers{
		health:     health.NewHandler(d.pool),
		webhook:    webhook.NewHandler(d.ch, svc, log),
		api:        apihandler.NewHandler(d.database, log),
		callback:   callback.NewHandler(d.database, d.ch, log),
		watchdog:   watchdog.NewHandler(d.database, log),
		githubAuth: githubauth.NewHandler(d.database, c.App.GithubAppSlug, log),
		upload:     uploadhandler.NewHandler(d.database, d.gcsClient, log),
		contextURL: contexturlhandler.NewHandler(d.database, d.ctxCache, d.ch, d.gcsClient, log),
		ctxTokens:  contexttokenshandler.NewHandler(d.database, log),
		aiKeys:     aikeyshandler.NewHandler(d.database, d.encSvc, log),
		query:      queryhandler.NewHandler(d.database, d.ctxCache, d.ch, d.gcsClient, d.encSvc, log),
		account:    accounthandler.NewHandler(d.database, log),
		org:        orghandler.NewHandler(d.database, log),
	}
}
