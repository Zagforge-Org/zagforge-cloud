package session

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// ListSessions returns the user's active sessions.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
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

	sessions, err := h.db.Queries.ListActiveSessions(r.Context(), userID)
	if err != nil {
		h.log.Error("list sessions", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]sessionResponse, len(sessions))
	for i, s := range sessions {
		result[i] = sessionResponse{
			ID:         httputil.UUIDToString(s.ID),
			LastActive: s.LastActiveAt.Time.Format(time.RFC3339),
			CreatedAt:  s.CreatedAt.Time.Format(time.RFC3339),
		}
		if s.IpAddress != nil {
			result[i].IPAddress = s.IpAddress.String()
		}
		if s.DeviceName.Valid {
			result[i].DeviceName = s.DeviceName.String
		}
		if s.Country.Valid {
			result[i].Country = s.Country.String
		}
	}

	httputil.OkResponse(w, result)
}

// RevokeSession revokes a specific session.
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionID")
	var sessionID pgtype.UUID
	if err := sessionID.Scan(sessionIDStr); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidSessionID)
		return
	}

	sess, err := h.db.Queries.GetSessionByID(r.Context(), sessionID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errSessionNotFound)
		return
	}

	userID, _ := claims.SubjectUUID()
	if sess.UserID != userID {
		httputil.ErrResponse(w, http.StatusForbidden, handler.ErrForbidden)
		return
	}

	_ = h.db.Queries.RevokeSession(r.Context(), sessionID)
	_ = h.db.Queries.RevokeRefreshTokensBySession(r.Context(), sessionID)

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "revoked"})
}
