package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type Handler struct {
	pool *pgxpool.Pool
}

type Response struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, Response{Status: "ok"})
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		httputil.WriteJSON(w, http.StatusServiceUnavailable, Response{Status: "unavailable", Reason: "db unreachable"})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, Response{Status: "ready"})
}
