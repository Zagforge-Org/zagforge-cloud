package org

import (
	"errors"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
)

var (
	errOrgNotFound       = errors.New("organization not found")
	errNotOwner          = errors.New("only the owner can perform this action")
	errCannotRemoveOwner = errors.New("cannot remove the org owner")
	errMemberNotFound    = errors.New("member not found")
)

type Handler struct {
	db       *db.DB
	auditSvc *audit.Service
	log      *zap.Logger
}

func NewHandler(db *db.DB, auditSvc *audit.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, auditSvc: auditSvc, log: log}
}
