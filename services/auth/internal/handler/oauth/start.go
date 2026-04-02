package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Start initiates the OAuth flow by generating state and redirecting to the provider.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		httputil.ErrResponse(w, http.StatusBadRequest, errUnsupportedProvider)
		return
	}

	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = h.frontendURL
	}

	state, err := generateState()
	if err != nil {
		h.log.Error("generate oauth state", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	err = h.db.Queries.CreateOAuthState(r.Context(), authstore.CreateOAuthStateParams{
		State:       state,
		Provider:    provider,
		RedirectUri: redirectURI,
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	})
	if err != nil {
		h.log.Error("store oauth state", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	authURL := p.AuthURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
