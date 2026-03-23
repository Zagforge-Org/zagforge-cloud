package api

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
)

const maxBranchLength = 256

var (
	ErrInvalidRepoID     = errors.New("invalid repo id")
	ErrInvalidJobID      = errors.New("invalid job id")
	ErrInvalidSnapshotID = errors.New("invalid snapshot id")
	ErrRepoNotFound      = errors.New("repo not found")
	ErrJobNotFound       = errors.New("job not found")
	ErrSnapshotNotFound  = errors.New("snapshot not found")
	ErrBranchRequired    = errors.New("branch query param required")
	ErrBranchTooLong     = errors.New("branch name exceeds maximum length")
	ErrInternal          = errors.New("internal error")
)

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

// verifyRepoOwnership checks that the repo exists and belongs to the requesting org.
func (h *Handler) verifyRepoOwnership(r *http.Request, repoID pgtype.UUID) error {
	repo, err := h.db.Queries.GetRepoByID(r.Context(), repoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRepoNotFound
		}
		return err
	}
	orgID := auth.OrgIDFromContext(r.Context())
	if repo.OrgID != orgID {
		return ErrRepoNotFound
	}
	return nil
}
