package aikeys

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

type Handler struct {
	db  *dbpkg.DB
	enc *encryption.Service
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, enc *encryption.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, enc: enc, log: log}
}

// List returns provider names and key hints — never the raw key.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())

	keys, err := h.db.Queries.ListAIProviderKeysByOrg(r.Context(), orgID)
	if err != nil {
		h.log.Error("list ai keys", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	httputil.OkResponse(w, keys)
}

// Upsert stores an encrypted AI provider key.
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())

	body, err := httputil.DecodeJSON[struct {
		Provider string `json:"provider" validate:"required"`
		RawKey   string `json:"raw_key" validate:"required,min=8"`
	}](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	cipher, err := h.enc.Encrypt([]byte(body.RawKey))
	if err != nil {
		h.log.Error("encrypt ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	hint := "..." + body.RawKey[len(body.RawKey)-4:]

	if _, err := h.db.Queries.UpsertAIProviderKeyForOrg(r.Context(), store.UpsertAIProviderKeyForOrgParams{
		OrgID:     orgID,
		Provider:  body.Provider,
		KeyCipher: cipher,
		KeyHint:   hint,
	}); err != nil {
		h.log.Error("upsert ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Delete removes an AI provider key.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := auth.OrgIDFromContext(r.Context())
	provider := chi.URLParam(r, "provider")

	if err := h.db.Queries.DeleteAIProviderKeyForOrg(r.Context(), store.DeleteAIProviderKeyForOrgParams{
		OrgID: orgID, Provider: provider,
	}); err != nil {
		h.log.Error("delete ai key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
