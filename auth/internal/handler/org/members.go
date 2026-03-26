package org

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/role"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// ListMembers returns all members of an organization.
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	members, err := h.db.Queries.ListOrgMembers(r.Context(), orgID)
	if err != nil {
		h.log.Error("list members", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]memberResponse, len(members))
	for i, m := range members {
		result[i] = toMemberResponse(m)
	}

	httputil.OkResponse(w, result)
}

// UpdateMemberRole updates a member's role. Requires owner or admin.
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

	// Cannot change owner's role via this endpoint.
	existing, err := h.db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: targetUserID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errMemberNotFound)
		return
	}
	if existing.Role == role.OrgOwner {
		httputil.ErrResponse(w, http.StatusForbidden, errCannotRemoveOwner)
		return
	}

	if err := h.db.Queries.UpdateOrgMemberRole(r.Context(), authstore.UpdateOrgMemberRoleParams{
		OrgID:  orgID,
		UserID: targetUserID,
		Role:   body.Role,
	}); err != nil {
		h.log.Error("update member role", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionOrgMemberRoleChanged,
		TargetType: audit.TargetTypeUser,
		TargetID:   targetUserID,
		Request:    r,
		Metadata:   fmt.Appendf(nil, `{"role":"%s"}`, body.Role),
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "updated"})
}

// RemoveMember removes a member from the organization. Requires owner or admin.
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

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	targetUserID, err := handler.ParseUUIDParam(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	// Cannot remove the owner.
	existing, err := h.db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: targetUserID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errMemberNotFound)
		return
	}
	if existing.Role == role.OrgOwner {
		httputil.ErrResponse(w, http.StatusForbidden, errCannotRemoveOwner)
		return
	}

	if err := h.db.Queries.DeleteOrgMembership(r.Context(), authstore.DeleteOrgMembershipParams{
		OrgID:  orgID,
		UserID: targetUserID,
	}); err != nil {
		h.log.Error("remove member", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionOrgMemberRemoved,
		TargetType: audit.TargetTypeUser,
		TargetID:   targetUserID,
		Request:    r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "removed"})
}

// TransferOwnership transfers org ownership to another member. Owner only.
func (h *Handler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
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

	if err := h.requireOwner(r, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[transferRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	var newOwnerID pgtype.UUID
	if err := newOwnerID.Scan(body.NewOwnerID); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidUserID)
		return
	}

	// Verify new owner is a member.
	if _, err := h.db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: newOwnerID,
	}); err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errMemberNotFound)
		return
	}

	// Promote new owner, demote current owner to admin.
	if err := h.db.Queries.UpdateOrgMemberRole(r.Context(), authstore.UpdateOrgMemberRoleParams{
		OrgID:  orgID,
		UserID: newOwnerID,
		Role:   role.OrgOwner,
	}); err != nil {
		h.log.Error("promote new owner", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	if err := h.db.Queries.UpdateOrgMemberRole(r.Context(), authstore.UpdateOrgMemberRoleParams{
		OrgID:  orgID,
		UserID: actorID,
		Role:   role.OrgAdmin,
	}); err != nil {
		h.log.Error("demote old owner", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionOrgOwnerTransferred,
		TargetType: audit.TargetTypeUser,
		TargetID:   newOwnerID,
		Request:    r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "transferred"})
}
