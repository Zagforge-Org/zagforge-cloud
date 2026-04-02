package session

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
)

var (
	errMissingRefreshToken = errors.New("missing refresh token")
	errInvalidRefreshToken = errors.New("invalid refresh token")
	errRefreshTokenExpired = errors.New("refresh token expired or revoked")
	errSessionExpired      = errors.New("session expired or revoked")
	errInvalidSessionID    = errors.New("invalid session id")
	errSessionNotFound     = errors.New("session not found")
)

type Handler struct {
	db         *db.DB
	tokenSvc   *token.Service
	sessionSvc *sessionsvc.Service
	auditSvc   *audit.Service
	log        *zap.Logger
}

func NewHandler(db *db.DB, tokenSvc *token.Service, sessionSvc *sessionsvc.Service, auditSvc *audit.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, tokenSvc: tokenSvc, sessionSvc: sessionSvc, auditSvc: auditSvc, log: log}
}
