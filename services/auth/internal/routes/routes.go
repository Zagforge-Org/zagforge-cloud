package routes

import (
	"crypto/ed25519"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

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
	"github.com/LegationPro/zagforge/shared/go/middleware/zaplogger"
	"github.com/LegationPro/zagforge/shared/go/middleware/zaprecoverer"
	"github.com/LegationPro/zagforge/shared/go/router"
)

// Deps holds everything the route layer needs to register handlers and middleware.
type Deps struct {
	Health  *health.Handler
	OAuth   *oauthhandler.Handler
	Session *sessionhandler.Handler
	User    *userhandler.Handler
	Org     *orghandler.Handler
	Invite  *invitehandler.Handler
	MFA     *mfahandler.Handler
	Team    *teamhandler.Handler
	Audit   *audithandler.Handler
	Webhook *webhookhandler.Handler
	Admin   *adminhandler.Handler

	RDB         *redis.Client
	PubKey      ed25519.PublicKey
	JWTIssuer   string
	CORSOrigins []string
	Log         *zap.Logger
}

// Register mounts global middleware and all route groups onto the router.
func Register(r *router.Router, d *Deps) error {
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(zaplogger.Middleware(d.Log))
	r.Use(zaprecoverer.Middleware(d.Log))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Org-Slug"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.ThrottleBacklog(100, 50, 5*time.Second))

	if err := registerHealth(r, d); err != nil {
		return err
	}
	if err := registerOAuth(r, d); err != nil {
		return err
	}
	if err := registerSession(r, d); err != nil {
		return err
	}
	if err := registerMFAPublic(r, d); err != nil {
		return err
	}
	return registerAuthenticated(r, d)
}
