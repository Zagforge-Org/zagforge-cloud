package clikeys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

type createResponse struct {
	ID        string    `json:"id"`
	RawKey    string    `json:"raw_key"`
	KeyHint   string    `json:"key_hint"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

type listItem struct {
	ID        string    `json:"id"`
	KeyHint   string    `json:"key_hint"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

// Create generates a new CLI API key for the current scope (personal or org).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := httputil.DecodeJSON[struct {
		Label string `json:"label" validate:"required"`
	}](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	raw, err := generateKey()
	if err != nil {
		h.log.Error("generate cli key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	hash := handlerpkg.SHA256Hash(raw)
	hint := raw[:len(clitoken.CLIKeyPrefix)+4] + "..." + raw[len(raw)-4:]

	params := store.InsertCLIAPIKeyParams{
		KeyHash: hash,
		KeyHint: hint,
		Label:   body.Label,
	}
	if auth.IsOrgScope(ctx) {
		params.OrgID = auth.OrgIDFromContext(ctx)
	} else {
		params.UserID = auth.UserIDFromContext(ctx)
	}

	key, err := h.db.Queries.InsertCLIAPIKey(ctx, params)
	if err != nil {
		h.log.Error("insert cli key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, createResponse{
		ID:        key.ID.String(),
		RawKey:    raw,
		KeyHint:   hint,
		Label:     key.Label,
		CreatedAt: key.CreatedAt.Time,
	})
}

// List returns all CLI API keys for the current scope (hints only, never raw keys).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var items []listItem

	if auth.IsOrgScope(ctx) {
		keys, err := h.db.Queries.ListCLIAPIKeysByOrg(ctx, auth.OrgIDFromContext(ctx))
		if err != nil {
			h.log.Error("list cli keys", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
			return
		}
		items = make([]listItem, len(keys))
		for i, k := range keys {
			items[i] = listItem{ID: k.ID.String(), KeyHint: k.KeyHint, Label: k.Label, CreatedAt: k.CreatedAt.Time}
		}
	} else {
		keys, err := h.db.Queries.ListCLIAPIKeysByUser(ctx, auth.UserIDFromContext(ctx))
		if err != nil {
			h.log.Error("list cli keys", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
			return
		}
		items = make([]listItem, len(keys))
		for i, k := range keys {
			items[i] = listItem{ID: k.ID.String(), KeyHint: k.KeyHint, Label: k.Label, CreatedAt: k.CreatedAt.Time}
		}
	}

	httputil.OkResponse(w, items)
}

// Delete revokes a CLI API key (must belong to caller's scope).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	keyID, err := httputil.ParseUUID(r, "keyID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if auth.IsOrgScope(ctx) {
		err = h.db.Queries.DeleteCLIAPIKeyForOrg(ctx, store.DeleteCLIAPIKeyForOrgParams{
			ID: keyID, OrgID: auth.OrgIDFromContext(ctx),
		})
	} else {
		err = h.db.Queries.DeleteCLIAPIKeyForUser(ctx, store.DeleteCLIAPIKeyForUserParams{
			ID: keyID, UserID: auth.UserIDFromContext(ctx),
		})
	}
	if err != nil {
		h.log.Error("delete cli key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func generateKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return clitoken.CLIKeyPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// ParseOrgID extracts the org_id from a CLI API key record.
// Returns an invalid UUID if the key is user-scoped (no org).
func ParseOrgID(key store.CliApiKey) pgtype.UUID {
	return key.OrgID
}
