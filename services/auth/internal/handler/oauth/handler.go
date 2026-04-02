package oauth

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/encryption"
	oauthsvc "github.com/LegationPro/zagforge/auth/internal/service/oauth"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
)

var (
	errUnsupportedProvider = errors.New("unsupported provider")
	errMissingCodeOrState  = errors.New("missing code or state")
	errInvalidState        = errors.New("invalid or expired state")
	errStateMismatch       = errors.New("state provider mismatch")
	errOAuthExchangeFailed = errors.New("oauth exchange failed")
)

type Handler struct {
	db          *db.DB
	providers   map[string]oauthsvc.Provider
	tokenSvc    *token.Service
	sessionSvc  *sessionsvc.Service
	encSvc      *encryption.Service
	auditSvc    *audit.Service
	log         *zap.Logger
	frontendURL string
	jwksKID     string
}

func NewHandler(
	db *db.DB,
	providers map[string]oauthsvc.Provider,
	tokenSvc *token.Service,
	sessionSvc *sessionsvc.Service,
	encSvc *encryption.Service,
	auditSvc *audit.Service,
	log *zap.Logger,
	frontendURL string,
	jwksKID string,
) *Handler {
	return &Handler{
		db:          db,
		providers:   providers,
		tokenSvc:    tokenSvc,
		sessionSvc:  sessionSvc,
		encSvc:      encSvc,
		auditSvc:    auditSvc,
		log:         log,
		frontendURL: frontendURL,
		jwksKID:     jwksKID,
	}
}
