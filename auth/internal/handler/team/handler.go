package team

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
)

var (
	errInvalidTeamID = errors.New("invalid team id")
	errTeamNotFound  = errors.New("team not found")
)

type Handler struct {
	db       *db.DB
	auditSvc *audit.Service
	log      *zap.Logger
}

func NewHandler(db *db.DB, auditSvc *audit.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, auditSvc: auditSvc, log: log}
}
