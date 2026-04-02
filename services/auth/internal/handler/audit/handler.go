package audit

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
)

var (
	errInvalidOrg  = errors.New("invalid org id")
	errInvalidDate = errors.New("invalid date format, expected RFC3339")
)

type Handler struct {
	db  *db.DB
	log *zap.Logger
}

func NewHandler(db *db.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}
