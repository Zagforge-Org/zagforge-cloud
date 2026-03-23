package auth

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

type orgIDKey struct{}

// OrgScope returns middleware that resolves the Clerk org from JWT claims,
// looks it up in the database, and stores the org ID in the request context.
// Rejects requests with no active org or unknown org.
func OrgScope(queries *store.Queries, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := ClaimsFromContext(r.Context())
			if err != nil {
				httputil.ErrResponse(w, http.StatusUnauthorized, ErrClaimsNotFound)
				return
			}

			clerkOrgID, err := ResolveClerkOrgID(claims)
			if err != nil {
				httputil.ErrResponse(w, http.StatusForbidden, ErrNoActiveOrg)
				return
			}

			org, err := queries.GetOrgByClerkID(r.Context(), clerkOrgID)
			if err != nil {
				log.Warn("org scope: org not found", zap.String("clerk_org_id", clerkOrgID))
				httputil.ErrResponse(w, http.StatusForbidden, ErrNoActiveOrg)
				return
			}

			ctx := context.WithValue(r.Context(), orgIDKey{}, org.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OrgIDFromContext retrieves the resolved org ID from request context.
// Returns an invalid UUID if not set (middleware not applied).
func OrgIDFromContext(ctx context.Context) pgtype.UUID {
	id, _ := ctx.Value(orgIDKey{}).(pgtype.UUID)
	return id
}
