package user

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var errCannotUnlinkOnly = errors.New("cannot unlink the only identity")

// ListIdentities returns the user's linked OAuth identities.
func (h *Handler) ListIdentities(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	identities, err := h.db.Queries.ListOAuthIdentitiesByUser(r.Context(), userID)
	if err != nil {
		h.log.Error("list identities", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]identityResponse, len(identities))
	for i, id := range identities {
		result[i] = identityResponse{
			Provider:   id.Provider,
			ProviderID: id.ProviderID,
		}
		if id.Email.Valid {
			result[i].Email = id.Email.String
		}
		if id.DisplayName.Valid {
			result[i].DisplayName = id.DisplayName.String
		}
		if id.AvatarUrl.Valid {
			result[i].AvatarURL = id.AvatarUrl.String
		}
	}

	httputil.OkResponse(w, result)
}

// UnlinkIdentity removes a linked OAuth provider from the user's account.
func (h *Handler) UnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	provider := chi.URLParam(r, "provider")

	count, err := h.db.Queries.CountOAuthIdentitiesByUser(r.Context(), userID)
	if err != nil {
		h.log.Error("count identities", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	if count <= 1 {
		httputil.ErrResponse(w, http.StatusBadRequest, errCannotUnlinkOnly)
		return
	}

	if err := h.db.Queries.DeleteOAuthIdentity(r.Context(), authstore.DeleteOAuthIdentityParams{
		UserID:   userID,
		Provider: provider,
	}); err != nil {
		h.log.Error("unlink identity", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "unlinked"})
}
