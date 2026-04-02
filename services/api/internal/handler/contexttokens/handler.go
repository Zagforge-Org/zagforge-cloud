package contexttokens

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

var (
	errRepoNotFound  = errors.New("repository not found")
	errInvalidExpiry = errors.New("expires_at must be a valid RFC3339 timestamp")
)

type createTokenResponse struct {
	ID         string                  `json:"id"`
	RawToken   string                  `json:"raw_token"`
	Label      string                  `json:"label,omitempty"`
	Visibility store.ContextVisibility `json:"visibility"`
	CreatedAt  time.Time               `json:"created_at"`
	ExpiresAt  *time.Time              `json:"expires_at,omitempty"`
}

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

// List returns context tokens for a repo.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())
	if err := handlerpkg.VerifyRepoOwnership(r.Context(), h.db.Queries, repoID, orgID); err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotFound)
		return
	}

	tokens, err := h.db.Queries.ListContextTokensByRepo(r.Context(), repoID)
	if err != nil {
		h.log.Error("list context tokens", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	httputil.OkResponse(w, tokens)
}

// Create generates a new context token and returns the raw token once only.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())

	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := handlerpkg.VerifyRepoOwnership(r.Context(), h.db.Queries, repoID, orgID); err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotFound)
		return
	}

	body, err := httputil.DecodeJSON[struct {
		Label        string   `json:"label"`
		ExpiresAt    *string  `json:"expires_at"`
		Visibility   string   `json:"visibility" validate:"omitempty,oneof=public private protected"`
		AllowedUsers []string `json:"allowed_users"`
	}](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	visibility := store.ContextVisibilityPublic
	if body.Visibility != "" {
		visibility = store.ContextVisibility(body.Visibility)
	}

	raw, err := generateToken()
	if err != nil {
		h.log.Error("generate token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	hash := handlerpkg.SHA256Hash(raw)

	var expiresAt pgtype.Timestamptz
	if body.ExpiresAt != nil {
		t, perr := time.Parse(time.RFC3339, *body.ExpiresAt)
		if perr != nil {
			httputil.ErrResponse(w, http.StatusBadRequest, errInvalidExpiry)
			return
		}
		expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	ctx := r.Context()

	tok, err := h.db.Queries.InsertContextToken(ctx, store.InsertContextTokenParams{
		RepoID:     repoID,
		OrgID:      orgID,
		TokenHash:  hash,
		Label:      pgtype.Text{String: body.Label, Valid: body.Label != ""},
		ExpiresAt:  expiresAt,
		Visibility: visibility,
	})
	if err != nil {
		h.log.Error("insert context token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	// If protected, insert allowed users.
	if visibility == store.ContextVisibilityProtected && len(body.AllowedUsers) > 0 {
		for _, uid := range body.AllowedUsers {
			userID, perr := httputil.UUIDFromString(uid)
			if perr != nil {
				continue
			}
			_ = h.db.Queries.InsertContextTokenAllowedUser(ctx, store.InsertContextTokenAllowedUserParams{
				TokenID: tok.ID,
				UserID:  userID,
			})
		}
	}

	resp := createTokenResponse{
		ID:         tok.ID.String(),
		RawToken:   raw,
		Label:      tok.Label.String,
		Visibility: tok.Visibility,
		CreatedAt:  tok.CreatedAt.Time,
	}
	if tok.ExpiresAt.Valid {
		t := tok.ExpiresAt.Time
		resp.ExpiresAt = &t
	}

	httputil.WriteJSON(w, http.StatusCreated, resp)
}

// UpdateAllowedUsers replaces the allowed user list for a protected context token.
func (h *Handler) UpdateAllowedUsers(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())

	tokenID, err := httputil.ParseUUID(r, "tokenID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	body, err := httputil.DecodeJSON[struct {
		UserIDs []string `json:"user_ids" validate:"required,min=1"`
	}](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	ctx := r.Context()
	_ = orgID

	// Clear existing allowed users and replace with new list.
	if err := h.db.Queries.ReplaceContextTokenAllowedUsers(ctx, tokenID); err != nil {
		h.log.Error("clear allowed users", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	// Insert new allowed users.
	for _, uid := range body.UserIDs {
		userID, perr := httputil.UUIDFromString(uid)
		if perr != nil {
			continue
		}
		if err := h.db.Queries.InsertContextTokenAllowedUser(ctx, store.InsertContextTokenAllowedUserParams{
			TokenID: tokenID,
			UserID:  userID,
		}); err != nil {
			h.log.Error("insert allowed user", zap.String("user_id", uid), zap.Error(err))
		}
	}

	_ = orgID // used for future ownership check

	w.WriteHeader(http.StatusNoContent)
}

// ListAllowedUsers returns the allowed users for a context token.
func (h *Handler) ListAllowedUsers(w http.ResponseWriter, r *http.Request) {
	tokenID, err := httputil.ParseUUID(r, "tokenID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	users, err := h.db.Queries.ListContextTokenAllowedUsers(r.Context(), tokenID)
	if err != nil {
		h.log.Error("list allowed users", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	httputil.OkResponse(w, users)
}

// Delete revokes a context token (must belong to caller's org).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())

	tokenID, err := httputil.ParseUUID(r, "tokenID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := h.db.Queries.DeleteContextTokenForOrg(r.Context(), store.DeleteContextTokenForOrgParams{
		ID: tokenID, OrgID: orgID,
	}); err != nil {
		h.log.Error("delete context token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
