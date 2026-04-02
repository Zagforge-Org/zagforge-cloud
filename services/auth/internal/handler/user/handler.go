package user

import (
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
)

type Handler struct {
	db  *db.DB
	log *zap.Logger
}

func NewHandler(db *db.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}
