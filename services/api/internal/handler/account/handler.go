package account

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

var errSessionNotFound = errors.New("session not found")

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

// GetProfile returns the authenticated user's profile and org memberships.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.db.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		h.log.Error("get user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	memberships, err := h.db.Queries.ListMembershipsByUser(r.Context(), userID)
	if err != nil {
		h.log.Error("list memberships", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.OkResponse(w, profileResponse{
		User:        user,
		Memberships: memberships,
	})
}

// UpdateProfile updates the authenticated user's username and/or phone.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	body, err := httputil.DecodeJSON[updateProfileRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}

	if body.Username == "" && body.Phone == nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errors.New("at least one field required"))
		return
	}

	user, err := h.db.Queries.UpdateUser(r.Context(), store.UpdateUserParams{
		Username:      body.Username,
		Email:         "",
		EmailVerified: false,
		Phone:         pgtype.Text{String: derefStr(body.Phone), Valid: body.Phone != nil},
		AvatarUrl:     pgtype.Text{},
		ID:            userID,
	})
	if err != nil {
		h.log.Error("update user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	if _, err := h.db.Queries.InsertAuditLog(r.Context(), store.InsertAuditLogParams{
		UserID:   userID,
		ActorID:  userID,
		Action:   "account.updated",
		TargetID: userID,
	}); err != nil {
		h.log.Warn("audit log write failed", zap.String("action", "account.updated"), zap.Error(err))
	}

	httputil.OkResponse(w, user)
}

// DeleteAccount deletes the authenticated user.
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	if err := h.db.Queries.DeleteUser(r.Context(), userID); err != nil {
		h.log.Error("delete user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSessions returns active sessions for the authenticated user.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	sessions, err := h.db.Queries.ListSessionsByUser(r.Context(), userID)
	if err != nil {
		h.log.Error("list sessions", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.OkResponse(w, sessions)
}

// RevokeSession terminates a specific session.
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	sessionID, err := httputil.ParseUUID(r, "id")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := h.db.Queries.DeleteSession(r.Context(), store.DeleteSessionParams{
		ID:     sessionID,
		UserID: userID,
	}); err != nil {
		h.log.Error("delete session", zap.Error(err))
		httputil.ErrResponse(w, http.StatusNotFound, errSessionNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type profileResponse struct {
	User        store.User                       `json:"user"`
	Memberships []store.ListMembershipsByUserRow `json:"memberships"`
}

type updateProfileRequest struct {
	Username string  `json:"username"`
	Phone    *string `json:"phone"`
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
