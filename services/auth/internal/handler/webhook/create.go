package webhook

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type createResponse struct {
	Webhook webhookResponse `json:"webhook"`
	Secret  string          `json:"secret"`
}

// Create registers a new webhook subscription.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[createWebhookRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	secret, err := generateSecret()
	if err != nil {
		h.log.Error("generate webhook secret", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	sub, err := h.db.Queries.CreateWebhookSubscription(r.Context(), authstore.CreateWebhookSubscriptionParams{
		OrgID:  orgID,
		Url:    body.URL,
		Secret: secret,
		Events: body.Events,
	})
	if err != nil {
		h.log.Error("create webhook", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Response[createResponse]{
		Data: createResponse{
			Webhook: toWebhookResponse(sub),
			Secret:  secret,
		},
	})
}
