package org

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

var (
	errOrgNotFound   = errors.New("organization not found")
	errAlreadyMember = errors.New("user is already a member")
	errUserNotFound  = errors.New("user not found")
	errLastOwner     = errors.New("cannot remove the last owner")
	errCannotDemote  = errors.New("only owners can change owner roles")
)

type Handler struct {
	db  *dbpkg.DB
	log *zap.Logger
}

func NewHandler(db *dbpkg.DB, log *zap.Logger) *Handler {
	return &Handler{db: db, log: log}
}

// requireRole checks that the caller has one of the allowed roles in the org.
func (h *Handler) requireRole(r *http.Request, orgID, userID pgtype.UUID, allowed ...string) (store.Membership, error) {
	m, err := h.db.Queries.GetMembership(r.Context(), store.GetMembershipParams{
		UserID: userID, OrgID: orgID,
	})
	if err != nil {
		return m, handlerpkg.ErrForbidden
	}
	if !slices.Contains(allowed, m.Role) {
		return m, handlerpkg.ErrForbidden
	}
	return m, nil
}

// CreateOrg creates a new organization and makes the caller the owner.
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	body, err := httputil.DecodeJSON[orgRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	// Auth org ID — placeholder until org is created via the auth service.
	org, err := h.db.Queries.UpsertOrg(r.Context(), store.UpsertOrgParams{
		AuthOrgID: fmt.Sprintf("local_%s", body.Slug),
		Slug:      body.Slug,
		Name:      body.Name,
	})
	if err != nil {
		h.log.Error("create org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	// Make caller the owner.
	_, err = h.db.Queries.CreateMembership(r.Context(), store.CreateMembershipParams{
		UserID: userID,
		OrgID:  org.ID,
		Role:   "owner",
	})
	if err != nil {
		h.log.Error("create owner membership", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	h.audit(r, userID, org.ID, "org.created", org.ID)
	httputil.WriteJSON(w, http.StatusCreated, org)
}

// ListOrgs returns organizations the authenticated user belongs to.
func (h *Handler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	memberships, err := h.db.Queries.ListMembershipsByUser(r.Context(), userID)
	if err != nil {
		h.log.Error("list orgs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.OkResponse(w, memberships)
}

// UpdateOrg updates an organization's name and/or slug. Requires owner or admin.
func (h *Handler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, userID, "owner", "admin"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	body, err := httputil.DecodeJSON[orgRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	org, err := h.db.Queries.UpdateOrganization(r.Context(), store.UpdateOrganizationParams{
		ID: orgID, Name: body.Name, Slug: body.Slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.ErrResponse(w, http.StatusNotFound, errOrgNotFound)
			return
		}
		h.log.Error("update org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	h.audit(r, pgtype.UUID{}, orgID, "org.updated", orgID)
	httputil.OkResponse(w, org)
}

// DeleteOrg deletes an organization. Requires owner.
func (h *Handler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, userID, "owner"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	if err := h.db.Queries.DeleteOrganization(r.Context(), orgID); err != nil {
		h.log.Error("delete org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns members of an organization. Requires any membership.
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, userID, "owner", "admin", "member"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	members, err := h.db.Queries.ListMembershipsByOrg(r.Context(), orgID)
	if err != nil {
		h.log.Error("list members", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.OkResponse(w, members)
}

// InviteMember adds a user to an organization by email. Requires owner or admin.
func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, userID, "owner", "admin"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	body, err := httputil.DecodeJSON[inviteRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	role := body.Role
	if role == "" {
		role = "member"
	}

	invitee, err := h.db.Queries.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.ErrResponse(w, http.StatusNotFound, errUserNotFound)
			return
		}
		h.log.Error("lookup invitee", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	membership, err := h.db.Queries.CreateMembership(r.Context(), store.CreateMembershipParams{
		UserID:    invitee.ID,
		OrgID:     orgID,
		Role:      role,
		InvitedBy: userID,
	})
	if err != nil {
		// Unique constraint violation means already a member.
		httputil.ErrResponse(w, http.StatusConflict, errAlreadyMember)
		return
	}

	h.audit(r, pgtype.UUID{}, orgID, "member.invited", invitee.ID)
	httputil.WriteJSON(w, http.StatusCreated, membership)
}

// UpdateMemberRole changes a member's role. Requires owner or admin.
// Admins cannot change owner roles — only owners can.
func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	targetUserID, err := httputil.ParseUUID(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	callerMembership, err := h.requireRole(r, orgID, callerID, "owner", "admin")
	if err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	body, err := httputil.DecodeJSON[roleRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handlerpkg.ErrInvalidBody)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	// Check target's current role.
	target, err := h.db.Queries.GetMembership(r.Context(), store.GetMembershipParams{
		UserID: targetUserID, OrgID: orgID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errUserNotFound)
		return
	}

	// Admins cannot modify owner roles.
	if callerMembership.Role == "admin" && (target.Role == "owner" || body.Role == "owner") {
		httputil.ErrResponse(w, http.StatusForbidden, errCannotDemote)
		return
	}

	// Prevent demoting the last owner.
	if target.Role == "owner" && body.Role != "owner" {
		count, err := h.db.Queries.CountOwnersByOrg(r.Context(), orgID)
		if err != nil {
			h.log.Error("count owners", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
			return
		}
		if count <= 1 {
			httputil.ErrResponse(w, http.StatusConflict, errLastOwner)
			return
		}
	}

	updated, err := h.db.Queries.UpdateMembershipRole(r.Context(), store.UpdateMembershipRoleParams{
		UserID: targetUserID, OrgID: orgID, Role: body.Role,
	})
	if err != nil {
		h.log.Error("update role", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	h.audit(r, pgtype.UUID{}, orgID, "member.role_changed", targetUserID)
	httputil.OkResponse(w, updated)
}

// RemoveMember removes a user from an organization. Requires owner or admin.
// Cannot remove the last owner.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	targetUserID, err := httputil.ParseUUID(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, callerID, "owner", "admin"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	// Check if target is the last owner.
	target, err := h.db.Queries.GetMembership(r.Context(), store.GetMembershipParams{
		UserID: targetUserID, OrgID: orgID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errUserNotFound)
		return
	}
	if target.Role == "owner" {
		count, err := h.db.Queries.CountOwnersByOrg(r.Context(), orgID)
		if err != nil {
			h.log.Error("count owners", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
			return
		}
		if count <= 1 {
			httputil.ErrResponse(w, http.StatusConflict, errLastOwner)
			return
		}
	}

	if err := h.db.Queries.DeleteMembership(r.Context(), store.DeleteMembershipParams{
		UserID: targetUserID, OrgID: orgID,
	}); err != nil {
		h.log.Error("remove member", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	h.audit(r, pgtype.UUID{}, orgID, "member.removed", targetUserID)
	w.WriteHeader(http.StatusNoContent)
}

// ListAuditLog returns the audit log for an organization. Requires owner or admin.
func (h *Handler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	orgID, err := httputil.ParseUUID(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if _, err := h.requireRole(r, orgID, userID, "owner", "admin"); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, handlerpkg.ErrForbidden)
		return
	}

	cursor, err := httputil.ParseCursor(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	limit := httputil.ParseLimit(r)

	entries, err := h.db.Queries.ListAuditLogByOrg(r.Context(), store.ListAuditLogByOrgParams{
		OrgID:     orgID,
		CreatedAt: cursor,
		Limit:     limit,
	})
	if err != nil {
		h.log.Error("list audit log", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	httputil.OkResponse(w, entries)
}

// audit writes an entry to the audit log. Errors are logged but do not fail the request.
func (h *Handler) audit(r *http.Request, userID, orgID pgtype.UUID, action string, targetID pgtype.UUID) {
	actorID := auth.UserIDFromContext(r.Context())
	if _, err := h.db.Queries.InsertAuditLog(r.Context(), store.InsertAuditLogParams{
		UserID:   userID,
		OrgID:    orgID,
		ActorID:  actorID,
		Action:   action,
		TargetID: targetID,
	}); err != nil {
		h.log.Warn("audit log write failed", zap.String("action", action), zap.Error(err))
	}
}

type orgRequest struct {
	Name string `json:"name" validate:"required"`
	Slug string `json:"slug" validate:"required"`
}

type inviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"omitempty,oneof=owner admin member"`
}

type roleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member"`
}
