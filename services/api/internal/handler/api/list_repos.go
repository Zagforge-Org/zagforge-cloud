package api

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/store"
)

func (h *Handler) ListRepos(w http.ResponseWriter, r *http.Request) {
	limit := httputil.ParseLimit(r)
	cursor := r.URL.Query().Get("cursor")

	if auth.IsOrgScope(r.Context()) {
		orgID := auth.OrgIDFromContext(r.Context())
		repos, err := h.db.Queries.ListReposByOrg(r.Context(), store.ListReposByOrgParams{
			OrgID:    orgID,
			FullName: cursor,
			Limit:    limit,
		})
		if err != nil {
			h.log.Error("list repos by org", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, ErrInternal)
			return
		}
		httputil.OkResponse(w, repos)
		return
	}

	userID := auth.UserIDFromContext(r.Context())
	repos, err := h.db.Queries.ListReposByUser(r.Context(), store.ListReposByUserParams{
		UserID:   userID,
		FullName: cursor,
		Limit:    limit,
	})
	if err != nil {
		h.log.Error("list repos by user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, ErrInternal)
		return
	}
	httputil.OkResponse(w, repos)
}
