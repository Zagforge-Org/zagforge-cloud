package session

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

func extractRefreshToken(r *http.Request) string {
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	body, err := httputil.DecodeJSON[refreshRequest](r.Body)
	if err == nil && body.RefreshToken != "" {
		return body.RefreshToken
	}
	return ""
}

func resolveOrgClaim(r *http.Request, queries *authstore.Queries, userID pgtype.UUID) authclaims.OrgClaim {
	orgSlug := r.Header.Get("X-Org-Slug")
	if orgSlug == "" {
		return authclaims.OrgClaim{}
	}

	org, err := queries.GetOrganizationBySlug(r.Context(), orgSlug)
	if err != nil {
		return authclaims.OrgClaim{}
	}

	membership, err := queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  org.ID,
		UserID: userID,
	})
	if err != nil {
		return authclaims.OrgClaim{}
	}

	return authclaims.OrgClaim{
		ID:   httputil.UUIDToString(org.ID),
		Slug: org.Slug,
		Role: membership.Role,
	}
}

func buildFullName(user authstore.User) string {
	name := ""
	if user.FirstName.Valid {
		name = user.FirstName.String
	}
	if user.LastName.Valid {
		name += " " + user.LastName.String
	}
	return strings.TrimSpace(name)
}

func isSecure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setRefreshCookie(w http.ResponseWriter, r *http.Request, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func setAccessCookie(w http.ResponseWriter, r *http.Request, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func clearAccessCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
