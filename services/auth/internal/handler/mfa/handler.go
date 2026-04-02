package mfa

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/encryption"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
)

var (
	errInvalidCode      = errors.New("invalid TOTP code")
	errInvalidBackup    = errors.New("invalid backup code")
	errMFANotEnabled    = errors.New("MFA is not enabled")
	errMFAAlreadyActive = errors.New("MFA is already enabled")
	errMFANotSetup      = errors.New("MFA not set up, call setup first")
	errInvalidToken     = errors.New("invalid or expired MFA challenge token")
)

type Handler struct {
	db         *db.DB
	tokenSvc   *token.Service
	sessionSvc *sessionsvc.Service
	encSvc     *encryption.Service
	auditSvc   *audit.Service
	log        *zap.Logger
}

func NewHandler(
	db *db.DB,
	tokenSvc *token.Service,
	sessionSvc *sessionsvc.Service,
	encSvc *encryption.Service,
	auditSvc *audit.Service,
	log *zap.Logger,
) *Handler {
	return &Handler{
		db:         db,
		tokenSvc:   tokenSvc,
		sessionSvc: sessionSvc,
		encSvc:     encSvc,
		auditSvc:   auditSvc,
		log:        log,
	}
}
