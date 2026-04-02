package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
)

const (
	deliveryTimeout    = 10 * time.Second
	maxAttempts        = 5
	maxResponseBodyLen = 1024
	signatureHeader    = "X-ZagForge-Signature"
	eventHeader        = "X-ZagForge-Event"
	signaturePrefix    = "sha256="
)

var errDelivery = errors.New("webhook delivery failed")

// Dispatcher sends webhook events to subscribed endpoints.
type Dispatcher struct {
	queries *authstore.Queries
	client  *http.Client
	log     *zap.Logger
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(queries *authstore.Queries, log *zap.Logger) *Dispatcher {
	return &Dispatcher{
		queries: queries,
		client:  &http.Client{Timeout: deliveryTimeout},
		log:     log,
	}
}

// Event represents a webhook event payload.
type Event struct {
	OrgID   pgtype.UUID
	Action  string
	Payload any
}

// Dispatch finds all active subscriptions for the event and creates delivery records.
func (d *Dispatcher) Dispatch(ctx context.Context, e Event) {
	subs, err := d.queries.ListActiveWebhooksByEvent(ctx, authstore.ListActiveWebhooksByEventParams{
		OrgID:  e.OrgID,
		Events: []string{e.Action},
	})
	if err != nil {
		d.log.Error("list webhook subscriptions", zap.Error(err))
		return
	}

	payload, err := json.Marshal(e.Payload)
	if err != nil {
		d.log.Error("marshal webhook payload", zap.Error(err))
		return
	}

	for _, sub := range subs {
		_, err := d.queries.CreateWebhookDelivery(ctx, authstore.CreateWebhookDeliveryParams{
			SubscriptionID: sub.ID,
			Event:          e.Action,
			Payload:        payload,
			Attempts:       0,
		})
		if err != nil {
			d.log.Error("create webhook delivery", zap.Error(err))
		}
	}
}

// DeliverPending processes pending webhook deliveries.
func (d *Dispatcher) DeliverPending(ctx context.Context, batchSize int32) {
	deliveries, err := d.queries.ListPendingDeliveries(ctx, batchSize)
	if err != nil {
		d.log.Error("list pending deliveries", zap.Error(err))
		return
	}

	for _, del := range deliveries {
		sub, err := d.queries.GetWebhookSubscription(ctx, del.SubscriptionID)
		if err != nil {
			d.log.Error("get webhook subscription", zap.Error(err))
			continue
		}

		d.processDelivery(ctx, sub, del)
	}
}

func (d *Dispatcher) processDelivery(ctx context.Context, sub authstore.WebhookSubscription, del authstore.WebhookDelivery) {
	status, body, err := d.deliver(ctx, sub, del)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	attempts := del.Attempts + 1

	if err == nil && isSuccessStatus(status) {
		_ = d.queries.UpdateWebhookDeliveryResult(ctx, authstore.UpdateWebhookDeliveryResultParams{
			ID:             del.ID,
			ResponseStatus: pgtype.Int4{Int32: int32(status), Valid: true},
			ResponseBody:   pgtype.Text{String: body, Valid: true},
			DeliveredAt:    now,
			Attempts:       attempts,
		})
		return
	}

	d.recordFailure(ctx, del.ID, status, body, attempts, now)
}

func (d *Dispatcher) recordFailure(ctx context.Context, deliveryID pgtype.UUID, status int, body string, attempts int32, now pgtype.Timestamptz) {
	var nextRetry pgtype.Timestamptz
	if attempts < maxAttempts {
		backoff := time.Duration(1<<uint(attempts)) * time.Minute
		nextRetry = pgtype.Timestamptz{Time: time.Now().Add(backoff), Valid: true}
	}

	respStatus := pgtype.Int4{}
	respBody := pgtype.Text{}
	if status > 0 {
		respStatus = pgtype.Int4{Int32: int32(status), Valid: true}
		respBody = pgtype.Text{String: body, Valid: true}
	}

	delivered := pgtype.Timestamptz{}
	if attempts >= maxAttempts {
		delivered = now
	}

	_ = d.queries.UpdateWebhookDeliveryResult(ctx, authstore.UpdateWebhookDeliveryResultParams{
		ID:             deliveryID,
		ResponseStatus: respStatus,
		ResponseBody:   respBody,
		DeliveredAt:    delivered,
		Attempts:       attempts,
		NextRetryAt:    nextRetry,
	})
}

func (d *Dispatcher) deliver(ctx context.Context, sub authstore.WebhookSubscription, del authstore.WebhookDelivery) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Url, bytes.NewReader(del.Payload))
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", errDelivery, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(eventHeader, del.Event)
	req.Header.Set(signatureHeader, Sign(del.Payload, sub.Secret))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", errDelivery, err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLen))
	return resp.StatusCode, string(bodyBytes), nil
}

// isSuccessStatus returns true for 2xx status codes.
func isSuccessStatus(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

// Sign creates an HMAC-SHA256 signature of the payload.
func Sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
