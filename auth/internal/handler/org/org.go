package org

import (
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

// Create creates a new organization and adds the user as owner.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	body, err := httputil.DecodeJSON[createOrgRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	org, err := h.db.Queries.CreateOrganization(r.Context(), authstore.CreateOrganizationParams{
		Slug: body.Slug,
		Name: body.Name,
	})
	if err != nil {
		h.log.Error("create org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	_, err = h.db.Queries.CreateOrgMembership(r.Context(), authstore.CreateOrgMembershipParams{
		OrgID:  org.ID,
		UserID: userID,
		Role:   role.OrgOwner,
	})
	if err != nil {
		h.log.Error("create owner membership", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   org.ID,
		ActorID: userID,
		Action:  audit.ActionOrgCreated,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusCreated, httputil.Response[orgResponse]{Data: toOrgResponse(org)})
}

// List returns all organizations the user belongs to.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgs, err := h.db.Queries.ListUserOrganizations(r.Context(), userID)
	if err != nil {
		h.log.Error("list orgs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]orgResponse, len(orgs))
	for i, o := range orgs {
		result[i] = toOrgResponse(o)
	}

	httputil.OkResponse(w, result)
}

// Get returns a single organization by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	org, err := h.db.Queries.GetOrganizationByID(r.Context(), orgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errOrgNotFound)
		return
	}

	httputil.OkResponse(w, toOrgResponse(org))
}

// Update updates an organization. Requires admin or owner role.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	if err := handler.RequireOrgAdminOrOwner(r, h.db, orgID, userID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	body, err := httputil.DecodeJSON[updateOrgRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	org, err := h.db.Queries.UpdateOrganization(r.Context(), authstore.UpdateOrganizationParams{
		ID:           orgID,
		Name:         body.Name,
		LogoUrl:      pgtype.Text{String: body.LogoURL, Valid: body.LogoURL != ""},
		BillingEmail: pgtype.Text{String: body.BillingEmail, Valid: body.BillingEmail != ""},
	})
	if err != nil {
		h.log.Error("update org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   orgID,
		ActorID: userID,
		Action:  audit.ActionOrgUpdated,
		Request: r,
	})

	httputil.OkResponse(w, toOrgResponse(org))
}

// Delete deletes an organization. Owner only.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, handler.ErrInvalidUserID)
		return
	}

	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	if err := h.requireOwner(r, orgID, userID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	if err := h.db.Queries.DeleteOrganization(r.Context(), orgID); err != nil {
		h.log.Error("delete org", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:   orgID,
		ActorID: userID,
		Action:  audit.ActionOrgDeleted,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "deleted"})
}

// requireOwner checks that the user is the org owner.
func (h *Handler) requireOwner(r *http.Request, orgID, userID pgtype.UUID) error {
	membership, err := h.db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: userID,
	})
	if err != nil {
		return handler.ErrForbidden
	}
	if membership.Role != role.OrgOwner {
		return errNotOwner
	}
	return nil
}
