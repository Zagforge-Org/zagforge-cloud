package webhook

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Update updates a webhook subscription.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	whID, err := handler.ParseUUIDParam(r, "whID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[updateWebhookRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	sub, err := h.db.Queries.UpdateWebhookSubscription(r.Context(), authstore.UpdateWebhookSubscriptionParams{
		ID:     whID,
		Url:    body.URL,
		Events: body.Events,
		Active: body.Active,
	})
	if err != nil {
		h.log.Error("update webhook", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, toWebhookResponse(sub))
}

// Delete removes a webhook subscription.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	whID, err := handler.ParseUUIDParam(r, "whID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	if err := h.db.Queries.DeleteWebhookSubscription(r.Context(), whID); err != nil {
		h.log.Error("delete webhook", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "deleted"})
}
