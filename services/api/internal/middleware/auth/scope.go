package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

type orgIDKey struct{}
type userIDKey struct{}

var (
	ErrNoActiveOrg  = errors.New("no active organization in session")
	ErrUserNotFound = errors.New("user not found")
)

// Scope returns middleware that resolves the active workspace from JWT claims
// and the X-Org-Slug request header.
//
// It ensures the authenticated user (and org, if applicable) exist in the API
// database via just-in-time sync from the auth service JWT claims.
func Scope(queries *store.Queries, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := authclaims.FromContext(r.Context())
			if err != nil {
				httputil.ErrResponse(w, http.StatusUnauthorized, ErrClaimsNotFound)
				return
			}

			// ── Resolve user (JIT sync) ──────────────────────────
			user, err := ensureUser(r.Context(), queries, claims)
			if err != nil {
				log.Error("scope: resolve user", zap.Error(err))
				httputil.ErrResponse(w, http.StatusInternalServerError, ErrUserNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, user.ID)

			// ── Resolve org scope ────────────────────────────────
			// Prefer X-Org-Slug header (set per-request by the frontend)
			// over the JWT org claim (fixed at token-issue time).
			orgSlug := r.Header.Get("X-Org-Slug")
			orgAuthID := r.Header.Get("X-Org-ID")
			orgName := r.Header.Get("X-Org-Name")

			// Fall back to JWT claims if headers are absent.
			if orgSlug == "" && claims.Org.Slug != "" {
				orgSlug = claims.Org.Slug
				orgAuthID = claims.Org.ID
			}

			if orgSlug != "" {
				org, err := ensureOrg(r.Context(), queries, orgSlug, orgAuthID, orgName)
				if err == nil {
					ctx = context.WithValue(ctx, orgIDKey{}, org.ID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ensureUser resolves or creates the user in the API database.
func ensureUser(ctx context.Context, q *store.Queries, claims *authclaims.Claims) (store.User, error) {
	user, err := q.GetUserByAuthID(ctx, claims.Subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return q.UpsertUser(ctx, store.UpsertUserParams{
			AuthUserID: claims.Subject,
			Username:   claims.Name,
			Email:      claims.Email,
		})
	}
	return user, err
}

// ensureOrg resolves the org by slug, creating it in the API DB if needed.
// authID and name come from the frontend headers or JWT claims.
func ensureOrg(ctx context.Context, q *store.Queries, slug, authID, name string) (store.Organization, error) {
	org, err := q.GetOrganizationBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) && authID != "" {
		// Org exists in auth DB but not API DB — JIT sync.
		if name == "" {
			name = slug
		}
		return q.UpsertOrg(ctx, store.UpsertOrgParams{
			AuthOrgID: authID,
			Slug:      slug,
			Name:      name,
		})
	}
	return org, err
}

// OrgIDFromContext retrieves the resolved org ID from request context.
// Returns an invalid UUID if the request is scoped to a personal workspace.
func OrgIDFromContext(ctx context.Context) pgtype.UUID {
	id, _ := ctx.Value(orgIDKey{}).(pgtype.UUID)
	return id
}

// UserIDFromContext retrieves the authenticated user's ID from request context.
// Always valid after the Scope middleware has run.
func UserIDFromContext(ctx context.Context) pgtype.UUID {
	id, _ := ctx.Value(userIDKey{}).(pgtype.UUID)
	return id
}

// IsOrgScope returns true if the current request is scoped to an organization.
func IsOrgScope(ctx context.Context) bool {
	return OrgIDFromContext(ctx).Valid
}
