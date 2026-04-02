package webhook

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type webhookResponse struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"org_id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"created_at"`
}

func toWebhookResponse(w authstore.WebhookSubscription) webhookResponse {
	return webhookResponse{
		ID:        httputil.UUIDToString(w.ID),
		OrgID:     httputil.UUIDToString(w.OrgID),
		URL:       w.Url,
		Events:    w.Events,
		Active:    w.Active,
		CreatedAt: w.CreatedAt.Time.Format(time.RFC3339),
	}
}

type deliveryResponse struct {
	ID             string `json:"id"`
	Event          string `json:"event"`
	ResponseStatus int32  `json:"response_status,omitempty"`
	Attempts       int32  `json:"attempts"`
	DeliveredAt    string `json:"delivered_at,omitempty"`
	CreatedAt      string `json:"created_at"`
}

func toDeliveryResponse(d authstore.WebhookDelivery) deliveryResponse {
	r := deliveryResponse{
		ID:        httputil.UUIDToString(d.ID),
		Event:     d.Event,
		Attempts:  d.Attempts,
		CreatedAt: d.CreatedAt.Time.Format(time.RFC3339),
	}
	if d.ResponseStatus.Valid {
		r.ResponseStatus = d.ResponseStatus.Int32
	}
	if d.DeliveredAt.Valid {
		r.DeliveredAt = d.DeliveredAt.Time.Format(time.RFC3339)
	}
	return r
}

type createWebhookRequest struct {
	URL    string   `json:"url" validate:"required,url,max=500"`
	Events []string `json:"events" validate:"required,min=1"`
}

type updateWebhookRequest struct {
	URL    string   `json:"url" validate:"required,url,max=500"`
	Events []string `json:"events" validate:"required,min=1"`
	Active bool     `json:"active"`
}
