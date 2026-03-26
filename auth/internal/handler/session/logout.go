package session

import (
	"net/http"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Logout revokes the current session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		tokenHash := token.HashToken(cookie.Value)
		rt, err := h.db.Queries.GetRefreshTokenByHash(r.Context(), tokenHash)
		if err == nil {
			_ = h.db.Queries.RevokeRefreshToken(r.Context(), rt.ID)
			_ = h.db.Queries.RevokeSession(r.Context(), rt.SessionID)
		}
	}

	clearRefreshCookie(w)

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "logged_out"})
}

// LogoutAll revokes all sessions for the authenticated user.
func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	userID, err := claims.SubjectUUID()
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	_ = h.db.Queries.RevokeAllUserSessions(r.Context(), userID)
	_ = h.db.Queries.RevokeAllUserRefreshTokens(r.Context(), userID)

	clearRefreshCookie(w)

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID: userID,
		Action:  audit.ActionUserLogoutAll,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "all_sessions_revoked"})
}
