package handler

import (
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/role"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
)

// UserIDFromContext extracts the authenticated user's UUID from JWT claims.
func UserIDFromContext(r *http.Request) (pgtype.UUID, error) {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		return pgtype.UUID{}, err
	}
	return claims.SubjectUUID()
}

// ParseOrgID extracts and parses the orgID URL parameter.
func ParseOrgID(r *http.Request) (pgtype.UUID, error) {
	return ParseUUIDParam(r, "orgID")
}

// ParseUUIDParam extracts and parses a UUID URL parameter by name.
func ParseUUIDParam(r *http.Request, param string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, param)); err != nil {
		return id, err
	}
	return id, nil
}

// RequireOrgAdminOrOwner checks that the user has admin or owner role in the org.
func RequireOrgAdminOrOwner(r *http.Request, db *db.DB, orgID, userID pgtype.UUID) error {
	membership, err := db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: userID,
	})
	if err != nil {
		return ErrForbidden
	}
	if !slices.Contains(role.OrgAdminOrAbove, membership.Role) {
		return ErrForbidden
	}
	return nil
}
