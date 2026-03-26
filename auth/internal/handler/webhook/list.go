package webhook

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// List returns all webhook subscriptions for an org.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	subs, err := h.db.Queries.ListWebhookSubscriptions(r.Context(), orgID)
	if err != nil {
		h.log.Error("list webhooks", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]webhookResponse, len(subs))
	for i, s := range subs {
		result[i] = toWebhookResponse(s)
	}
	httputil.OkResponse(w, result)
}

// ListDeliveries returns delivery history for a webhook.
func (h *Handler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	whID, err := handler.ParseUUIDParam(r, "whID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	deliveries, err := h.db.Queries.ListWebhookDeliveries(r.Context(), authstore.ListWebhookDeliveriesParams{
		SubscriptionID: whID,
		Limit:          defaultDeliveryLimit,
	})
	if err != nil {
		h.log.Error("list deliveries", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]deliveryResponse, len(deliveries))
	for i, d := range deliveries {
		result[i] = toDeliveryResponse(d)
	}
	httputil.OkResponse(w, result)
}
