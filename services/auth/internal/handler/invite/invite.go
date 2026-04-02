package invite

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

const inviteTTL = 7 * 24 * time.Hour // 7 days

// Create creates a new invite. Requires admin or owner role.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[createInviteRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	// Check member limit.
	org, err := h.db.Queries.GetOrganizationByID(r.Context(), orgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	count, err := h.db.Queries.CountOrgMembers(r.Context(), orgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	if int32(count) >= org.MaxMembers {
		httputil.ErrResponse(w, http.StatusForbidden, errMaxMembers)
		return
	}

	rawToken, err := generateInviteToken()
	if err != nil {
		h.log.Error("generate invite token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	inv, err := h.db.Queries.CreateInvite(r.Context(), authstore.CreateInviteParams{
		OrgID:     orgID,
		InvitedBy: actorID,
		Email:     body.Email,
		Role:      body.Role,
		TokenHash: token.HashToken(rawToken),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(inviteTTL), Valid: true},
	})
	if err != nil {
		h.log.Error("create invite", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   orgID,
		ActorID: actorID,
		Action:  audit.ActionInviteCreated,
		Request: r,
	})

	// Return the raw token only once — client must save it.
	type createResponse struct {
		Invite inviteResponse `json:"invite"`
		Token  string         `json:"token"`
	}
	httputil.WriteJSON(w, http.StatusCreated, httputil.Response[createResponse]{
		Data: createResponse{
			Invite: toInviteResponse(inv),
			Token:  rawToken,
		},
	})
}

// ListOrgInvites returns all invites for an org.
func (h *Handler) ListOrgInvites(w http.ResponseWriter, r *http.Request) {
	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	invites, err := h.db.Queries.ListOrgInvites(r.Context(), orgID)
	if err != nil {
		h.log.Error("list invites", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]inviteResponse, len(invites))
	for i, inv := range invites {
		result[i] = toInviteResponse(inv)
	}

	httputil.OkResponse(w, result)
}

// Revoke revokes a pending invite. Requires admin or owner.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	actorID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	var inviteID pgtype.UUID
	if err := inviteID.Scan(chi.URLParam(r, "inviteID")); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidInviteID)
		return
	}

	if err := h.db.Queries.RevokeInvite(r.Context(), authstore.RevokeInviteParams{
		ID:    inviteID,
		OrgID: orgID,
	}); err != nil {
		h.log.Error("revoke invite", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   orgID,
		ActorID: actorID,
		Action:  audit.ActionInviteRevoked,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "revoked"})
}

// Accept accepts an invite by token. The authenticated user is added to the org.
func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	body, err := httputil.DecodeJSON[acceptInviteRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	tokenHash := token.HashToken(body.Token)

	inv, err := h.db.Queries.GetInviteByTokenHash(r.Context(), tokenHash)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidToken)
		return
	}

	// Check member limit.
	org, err := h.db.Queries.GetOrganizationByID(r.Context(), inv.OrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	count, err := h.db.Queries.CountOrgMembers(r.Context(), inv.OrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	if int32(count) >= org.MaxMembers {
		httputil.ErrResponse(w, http.StatusForbidden, errMaxMembers)
		return
	}

	// Mark invite as accepted.
	if _, err := h.db.Queries.AcceptInvite(r.Context(), inv.ID); err != nil {
		h.log.Error("accept invite", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	// Add user as member.
	_, err = h.db.Queries.CreateOrgMembership(r.Context(), authstore.CreateOrgMembershipParams{
		OrgID:  inv.OrgID,
		UserID: userID,
		Role:   inv.Role,
	})
	if err != nil {
		h.log.Error("create membership from invite", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   inv.OrgID,
		ActorID: userID,
		Action:  audit.ActionInviteAccepted,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "accepted"})
}

// GetByToken returns invite details by token. Public endpoint (no auth required).
func (h *Handler) GetByToken(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	tokenHash := token.HashToken(rawToken)

	inv, err := h.db.Queries.GetInviteByTokenHash(r.Context(), tokenHash)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errInvalidToken)
		return
	}

	org, err := h.db.Queries.GetOrganizationByID(r.Context(), inv.OrgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	type publicInviteResponse struct {
		OrgName   string `json:"org_name"`
		OrgSlug   string `json:"org_slug"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		ExpiresAt string `json:"expires_at"`
	}

	httputil.OkResponse(w, publicInviteResponse{
		OrgName:   org.Name,
		OrgSlug:   org.Slug,
		Email:     inv.Email,
		Role:      inv.Role,
		ExpiresAt: inv.ExpiresAt.Time.Format(time.RFC3339),
	})
}

func generateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
