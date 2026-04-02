package team

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Create creates a new team within an org. Requires org admin or owner.
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

	body, err := httputil.DecodeJSON[createTeamRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	team, err := h.db.Queries.CreateTeam(r.Context(), authstore.CreateTeamParams{
		OrgID:       orgID,
		Slug:        body.Slug,
		Name:        body.Name,
		Description: pgtype.Text{String: body.Description, Valid: body.Description != ""},
	})
	if err != nil {
		h.log.Error("create team", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionTeamCreated,
		TargetType: audit.TargetTypeTeam,
		TargetID:   team.ID,
		Request:    r,
	})

	httputil.WriteJSON(w, http.StatusCreated, httputil.Response[teamResponse]{Data: toTeamResponse(team)})
}

// List returns all teams in an org.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := handler.ParseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, handler.ErrInvalidOrgID)
		return
	}

	teams, err := h.db.Queries.ListTeamsByOrg(r.Context(), orgID)
	if err != nil {
		h.log.Error("list teams", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	result := make([]teamResponse, len(teams))
	for i, t := range teams {
		result[i] = toTeamResponse(t)
	}
	httputil.OkResponse(w, result)
}

// Get returns a single team.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	teamID, err := handler.ParseUUIDParam(r, "teamID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidTeamID)
		return
	}

	team, err := h.db.Queries.GetTeamByID(r.Context(), teamID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errTeamNotFound)
		return
	}

	httputil.OkResponse(w, toTeamResponse(team))
}

// Update updates a team. Requires org admin or owner.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
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

	body, err := httputil.DecodeJSON[updateTeamRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	team, err := h.db.Queries.UpdateTeam(r.Context(), authstore.UpdateTeamParams{
		ID:          teamID,
		Name:        body.Name,
		Slug:        body.Slug,
		Description: pgtype.Text{String: body.Description, Valid: body.Description != ""},
	})
	if err != nil {
		h.log.Error("update team", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionTeamUpdated,
		TargetType: audit.TargetTypeTeam,
		TargetID:   teamID,
		Request:    r,
	})

	httputil.OkResponse(w, toTeamResponse(team))
}

// Delete removes a team. Requires org admin or owner.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.db.Queries.DeleteTeam(r.Context(), teamID); err != nil {
		h.log.Error("delete team", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     audit.ActionTeamDeleted,
		TargetType: audit.TargetTypeTeam,
		TargetID:   teamID,
		Request:    r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "deleted"})
}
