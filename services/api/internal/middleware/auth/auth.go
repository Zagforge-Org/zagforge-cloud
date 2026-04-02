package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var (
	ErrMissingToken   = errors.New("missing authorization token")
	ErrInvalidToken   = errors.New("invalid or expired token")
	ErrClaimsNotFound = errors.New("auth claims not found in context")
)

// ClaimsFromContext retrieves the auth claims from the request context.
func ClaimsFromContext(ctx context.Context) (*authclaims.Claims, error) {
	return authclaims.FromContext(ctx)
}

// Auth returns middleware that verifies Ed25519 JWTs on incoming requests.
func Auth(pubKey ed25519.PublicKey, issuer string, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				httputil.ErrResponse(w, http.StatusUnauthorized, ErrMissingToken)
				return
			}

			claims := &authclaims.Claims{}
			t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return pubKey, nil
			}, jwt.WithIssuer(issuer))
			if err != nil || !t.Valid {
				log.Warn("auth: invalid token", zap.Error(err))
				httputil.ErrResponse(w, http.StatusUnauthorized, ErrInvalidToken)
				return
			}

			ctx := authclaims.NewContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	// Prefer Authorization header (client-side requests, CLI tools).
	if token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); found {
		return token
	}
	// Fall back to HttpOnly cookie (browser requests from Next.js dashboard).
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}
