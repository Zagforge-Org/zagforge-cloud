package routes

import (
	"crypto/ed25519"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

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
	"github.com/LegationPro/zagforge/shared/go/jobtoken"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaplogger"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaprecoverer"
	"github.com/LegationPro/zagforge/shared/go/router"
	"github.com/LegationPro/zagforge/shared/go/store"
)

// Deps holds everything the route layer needs to register handlers and middleware.
type Deps struct {
	Health     *health.Handler
	Webhook    *webhook.Handler
	API        *apihandler.Handler
	Callback   *callback.Handler
	Watchdog   *watchdog.Handler
	GithubAuth *githubauth.Handler
	Upload     *uploadhandler.Handler
	ContextURL *contexturlhandler.Handler
	CtxTokens  *contexttokenshandler.Handler
	AIKeys     *aikeyshandler.Handler
	CLIKeys    *clikeyshandler.Handler
	Query      *queryhandler.Handler
	Account    *accounthandler.Handler
	Org        *orghandler.Handler

	Queries        *store.Queries
	RDB            *redis.Client
	JWTPubKey      ed25519.PublicKey
	JWTIssuer      string
	Signer         *jobtoken.Signer
	CORSOrigins    []string
	WatchdogSecret string
	CLIAPIKey      string
	Log            *zap.Logger
}

// Register mounts global middleware and all route groups onto the router.
func Register(r *router.Router, d *Deps) error {
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zaplogger.Middleware(d.Log))
	r.Use(zaprecoverer.Middleware(d.Log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.ThrottleBacklog(100, 50, 5*time.Second))

	if err := registerHealth(r, d); err != nil {
		return err
	}
	if err := registerGithubAuth(r, d); err != nil {
		return err
	}
	if err := registerInternal(r, d); err != nil {
		return err
	}
	if err := registerWatchdog(r, d); err != nil {
		return err
	}
	if err := registerAPIv1(r, d); err != nil {
		return err
	}
	if err := registerContext(r, d); err != nil {
		return err
	}
	return registerUpload(r, d)
}
