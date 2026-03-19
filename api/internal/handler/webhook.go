package handler

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"

	github "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
)

const maxPayloadBytes = 25 * 1024 * 1024

var _ http.Handler = (*WebhookHandler)(nil)

var supportedEvents = map[string]bool{
	"push": true,
}

// pushHandler receives a validated push event and delivery ID.
// JobService satisfies this interface.
type pushHandler interface {
	HandlePush(ctx context.Context, event github.WebhookEvent, deliveryID string) error
}

type WebhookHandler struct {
	validator github.WebhookValidator
	svc       pushHandler
}

func NewWebhookHandler(v github.WebhookValidator, svc pushHandler) *WebhookHandler {
	return &WebhookHandler{validator: v, svc: svc}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadBytes+1))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	if int64(len(body)) > maxPayloadBytes {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	event, err := h.validator.ValidateWebhook(r.Context(), body, signature, eventType)
	if errors.Is(err, github.ErrInvalidSignature) {
		log.Printf("webhook: invalid signature event=%s", eventType)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("webhook: validation error event=%s: %v", eventType, err)
		http.Error(w, "validation error", http.StatusInternalServerError)
		return
	}

	if !supportedEvents[event.EventType] {
		log.Printf("webhook: ignoring unsupported event=%s", event.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	log.Printf("webhook: dispatching event=%s repo=%s branch=%s commit=%s",
		event.EventType, event.RepoName, event.Branch, event.CommitSHA)

	if err := h.svc.HandlePush(r.Context(), event, deliveryID); err != nil {
		log.Printf("webhook: handle push error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
