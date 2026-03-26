package audit

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
)

var (
	errInternal    = errors.New("internal error")
	errInvalidOrg  = errors.New("invalid org id")
	errForbidden   = errors.New("admin or owner role required")
	errInvalidDate = errors.New("invalid date format, expected RFC3339")
)

type Handler struct {
	db  *db.DB
	log *zap.Logger
}

func NewHandler(db *db.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}
