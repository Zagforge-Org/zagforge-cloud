package audit

import (
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// List returns audit logs for an org, paginated by cursor.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidOrg)
		return
	}

	actorID, err := userIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	if err := h.requireOrgAdminOrOwner(r, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	cursor := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httputil.ErrResponse(w, http.StatusBadRequest, errInvalidDate)
			return
		}
		cursor = pgtype.Timestamptz{Time: t, Valid: true}
	}

	limit := int32(50)
	action := r.URL.Query().Get("action")

	var logs []authstore.AuditLog
	if action != "" {
		logs, err = h.db.Queries.ListAuditLogsByAction(r.Context(), authstore.ListAuditLogsByActionParams{
			OrgID:     orgID,
			Action:    action,
			CreatedAt: cursor,
			Limit:     limit,
		})
	} else {
		logs, err = h.db.Queries.ListAuditLogs(r.Context(), authstore.ListAuditLogsParams{
			OrgID:     orgID,
			CreatedAt: cursor,
			Limit:     limit,
		})
	}
	if err != nil {
		h.log.Error("list audit logs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	result := make([]auditLogResponse, len(logs))
	for i, l := range logs {
		result[i] = toAuditLogResponse(l)
	}

	var nextCursor *string
	if len(logs) == int(limit) {
		c := logs[len(logs)-1].CreatedAt.Time.Format(time.RFC3339Nano)
		nextCursor = &c
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Response[[]auditLogResponse]{
		Data:       result,
		NextCursor: nextCursor,
	})
}

// LoginMetrics returns daily login counts for an org.
func (h *Handler) LoginMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidOrg)
		return
	}

	actorID, err := userIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	if err := h.requireOrgAdminOrOwner(r, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	from, to := parseDateRange(r)

	rows, err := h.db.Queries.CountLoginsByDay(r.Context(), authstore.CountLoginsByDayParams{
		OrgID:       orgID,
		CreatedAt:   from,
		CreatedAt_2: to,
	})
	if err != nil {
		h.log.Error("login metrics", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	result := make([]loginMetricRow, len(rows))
	for i, row := range rows {
		result[i] = loginMetricRow{
			Day:   row.Day.Time.Format("2006-01-02"),
			Total: row.Total,
		}
	}

	httputil.OkResponse(w, result)
}

// FailedLoginMetrics returns daily failed login counts.
func (h *Handler) FailedLoginMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidOrg)
		return
	}

	actorID, err := userIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	if err := h.requireOrgAdminOrOwner(r, orgID, actorID); err != nil {
		httputil.ErrResponse(w, http.StatusForbidden, err)
		return
	}

	from, to := parseDateRange(r)

	rows, err := h.db.Queries.CountFailedLoginsByDay(r.Context(), authstore.CountFailedLoginsByDayParams{
		CreatedAt:   from,
		CreatedAt_2: to,
	})
	if err != nil {
		h.log.Error("failed login metrics", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	result := make([]loginMetricRow, len(rows))
	for i, row := range rows {
		result[i] = loginMetricRow{
			Day:   row.Day.Time.Format("2006-01-02"),
			Total: row.Total,
		}
	}

	httputil.OkResponse(w, result)
}

func parseOrgID(r *http.Request) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "orgID")); err != nil {
		return id, err
	}
	return id, nil
}

func userIDFromContext(r *http.Request) (pgtype.UUID, error) {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		return pgtype.UUID{}, err
	}
	return claims.SubjectUUID()
}

func (h *Handler) requireOrgAdminOrOwner(r *http.Request, orgID, userID pgtype.UUID) error {
	membership, err := h.db.Queries.GetOrgMembership(r.Context(), authstore.GetOrgMembershipParams{
		OrgID:  orgID,
		UserID: userID,
	})
	if err != nil {
		return errForbidden
	}
	if !slices.Contains([]string{"owner", "admin"}, membership.Role) {
		return errForbidden
	}
	return nil
}

func parseDateRange(r *http.Request) (pgtype.Timestamptz, pgtype.Timestamptz) {
	from := pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -30), Valid: true}
	to := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	if raw := r.URL.Query().Get("from"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			from = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			to = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Second), Valid: true}
		}
	}

	return from, to
}
