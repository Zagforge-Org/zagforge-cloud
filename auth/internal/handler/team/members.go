package team

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// ListMembers returns all members of a team.
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParseUUIDParam(r, "teamID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidTeamID)
		return
	}

	members, err := h.db.Queries.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		h.log.Error("list team members", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]memberResponse, len(members))
	for i, m := range members {
		result[i] = toMemberResponse(m)
	}
	httputil.OkResponse(w, result)
}

// AddMember adds a user to a team. Requires org admin or owner.
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
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

	teamID, err := handler.ParseUUIDParam(r, "teamID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidTeamID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[addMemberRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	var targetUserID pgtype.UUID
	if err := targetUserID.Scan(body.UserID); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	_, err = h.db.Queries.CreateTeamMembership(r.Context(), authstore.CreateTeamMembershipParams{
		TeamID: teamID,
		UserID: targetUserID,
		Role:   body.Role,
	})
	if err != nil {
		h.log.Error("add team member", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionTeamMemberAdded,
		TargetType: audit.TargetTypeUser,
		TargetID:   targetUserID,
		Request:    r,
		Metadata:   fmt.Appendf(nil, `{"team_id":"%s","role":"%s"}`, httputil.UUIDToString(teamID), body.Role),
	})

	httputil.WriteJSON(w, http.StatusCreated, handler.StatusResponse{Status: "added"})
}

// RemoveMember removes a user from a team. Requires org admin or owner.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
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

	teamID, err := handler.ParseUUIDParam(r, "teamID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidTeamID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	targetUserID, err := handler.ParseUUIDParam(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	if err := h.db.Queries.DeleteTeamMembership(r.Context(), authstore.DeleteTeamMembershipParams{
		TeamID: teamID,
		UserID: targetUserID,
	}); err != nil {
		h.log.Error("remove team member", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionTeamMemberRemoved,
		TargetType: audit.TargetTypeUser,
		TargetID:   targetUserID,
		Request:    r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "removed"})
}

// UpdateMemberRole changes a team member's role. Requires org admin or owner.
func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
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

	teamID, err := handler.ParseUUIDParam(r, "teamID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidTeamID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	targetUserID, err := handler.ParseUUIDParam(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	body, err := httputil.DecodeJSON[updateRoleRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := h.db.Queries.UpdateTeamMemberRole(r.Context(), authstore.UpdateTeamMemberRoleParams{
		TeamID: teamID,
		UserID: targetUserID,
		Role:   body.Role,
	}); err != nil {
		h.log.Error("update team member role", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "updated"})
}
