package clitoken

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

type cliOrgIDKey struct{}

// CLIKeyPrefix is the prefix for per-org CLI API keys.
var CLIKeyPrefix = "zf_cli_"

var (
	errMissing  = errors.New("missing CLI API key")
	errInvalid  = errors.New("invalid CLI API key")
	errInternal = errors.New("internal error")
)

// Auth returns middleware that validates a CLI bearer token.
//
// It first tries to look up the token as a per-org key in the database.
// If no match is found, it falls back to the global static key (for backward compatibility).
// When a per-org key matches, the resolved org_id is injected into the request context.
func Auth(queries *store.Queries, globalKey string, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !found || token == "" {
				httputil.ErrResponse(w, http.StatusUnauthorized, errMissing)
				return
			}

			// Try per-org key lookup first.
			if strings.HasPrefix(token, CLIKeyPrefix) {
				hash := handlerpkg.SHA256Hash(token)
				key, err := queries.GetCLIAPIKeyByHash(r.Context(), hash)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						httputil.ErrResponse(w, http.StatusUnauthorized, errInvalid)
						return
					}
					log.Error("cli auth: db lookup failed", zap.Error(err))
					httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
					return
				}

				ctx := context.WithValue(r.Context(), cliOrgIDKey{}, key.OrgID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fallback: global static key.
			if globalKey != "" && subtle.ConstantTimeCompare([]byte(token), []byte(globalKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			httputil.ErrResponse(w, http.StatusUnauthorized, errInvalid)
		})
	}
}

// OrgIDFromCLIKey retrieves the org_id resolved from a per-org CLI key.
// Returns an invalid UUID if the request used the global key fallback.
func OrgIDFromCLIKey(ctx context.Context) pgtype.UUID {
	id, _ := ctx.Value(cliOrgIDKey{}).(pgtype.UUID)
	return id
}

// HasOrgScope returns true if the CLI key resolved to a specific org.
func HasOrgScope(ctx context.Context) bool {
	return OrgIDFromCLIKey(ctx).Valid
}
