package admin

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
)

var (
	errForbidden    = errors.New("platform admin required")
	errInvalidID    = errors.New("invalid id")
	errUserNotFound = errors.New("user not found")
)

type Handler struct {
	db  *db.DB
	log *zap.Logger
}

func NewHandler(db *db.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}
