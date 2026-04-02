package admin

import (
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// ListUsers returns all users (paginated). Platform admin only.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformAdmin(r); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	limit, offset := parsePagination(r)

	users, err := h.db.Queries.ListUsers(r.Context(), authstore.ListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.log.Error("list users", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	total, err := h.db.Queries.CountUsers(r.Context())
	if err != nil {
		h.log.Error("count users", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]userResponse, len(users))
	for i, u := range users {
		result[i] = toUserResponse(u)
	}

	httputil.OkResponse(w, listResponse[userResponse]{Items: result, Total: total})
}

// GetUser returns a single user. Platform admin only.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformAdmin(r); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	userID, err := handler.ParseUUIDParam(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	user, err := h.db.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errUserNotFound)
		return
	}

	httputil.OkResponse(w, toUserResponse(user))
}

// UpdateUser updates admin-level user fields. Platform admin only.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformAdmin(r); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	userID, err := handler.ParseUUIDParam(r, "userID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	body, err := httputil.DecodeJSON[updateUserRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if body.IsPlatformAdmin != nil {
		if err := h.db.Queries.UpdateUserPlatformAdmin(r.Context(), authstore.UpdateUserPlatformAdminParams{
			ID:              userID,
			IsPlatformAdmin: *body.IsPlatformAdmin,
		}); err != nil {
			h.log.Error("update platform admin", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
			return
		}
	}

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "updated"})
}

// ListOrgs returns all organizations (paginated). Platform admin only.
func (h *Handler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformAdmin(r); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	limit, offset := parsePagination(r)

	orgs, err := h.db.Queries.ListOrganizations(r.Context(), authstore.ListOrganizationsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.log.Error("list orgs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	total, err := h.db.Queries.CountOrganizations(r.Context())
	if err != nil {
		h.log.Error("count orgs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]orgResponse, len(orgs))
	for i, o := range orgs {
		result[i] = toOrgResponse(o)
	}

	httputil.OkResponse(w, listResponse[orgResponse]{Items: result, Total: total})
}

// UpdateOrgPlan updates an org's plan and member limit. Platform admin only.
func (h *Handler) UpdateOrgPlan(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformAdmin(r); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	orgID, err := handler.ParseUUIDParam(r, "orgID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidID)
		return
	}

	body, err := httputil.DecodeJSON[updateOrgPlanRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	org, err := h.db.Queries.UpdateOrganizationPlan(r.Context(), authstore.UpdateOrganizationPlanParams{
		ID:         orgID,
		Plan:       body.Plan,
		MaxMembers: body.MaxMembers,
	})
	if err != nil {
		h.log.Error("update org plan", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, toOrgResponse(org))
}

func (h *Handler) requirePlatformAdmin(r *http.Request) error {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		return errForbidden
	}

	userID, err := claims.SubjectUUID()
	if err != nil {
		return errForbidden
	}

	user, err := h.db.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		return errForbidden
	}

	if !user.IsPlatformAdmin {
		return errForbidden
	}

	return nil
}

func parsePagination(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)

	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	return limit, offset
}
