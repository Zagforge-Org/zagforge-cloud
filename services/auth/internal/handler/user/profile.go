package user

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// GetMe returns the authenticated user's profile.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	user, err := h.db.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		h.log.Error("get user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, toProfileResponse(user))
}

// UpdateMe updates the authenticated user's profile.
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	var body updateProfileRequest
	body, err = httputil.DecodeJSON[updateProfileRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	socialLinks, _ := json.Marshal(body.SocialLinks)
	if body.SocialLinks == nil {
		socialLinks = []byte("{}")
	}

	visibility := body.ProfileVisibility
	if visibility == "" {
		visibility = "team_only"
	}

	tz := body.Timezone
	if tz == "" {
		tz = "UTC"
	}

	lang := body.Language
	if lang == "" {
		lang = "en"
	}

	user, err := h.db.Queries.UpdateUserProfile(r.Context(), authstore.UpdateUserProfileParams{
		ID:                userID,
		FirstName:         pgtype.Text{String: body.FirstName, Valid: body.FirstName != ""},
		LastName:          pgtype.Text{String: body.LastName, Valid: body.LastName != ""},
		Nickname:          pgtype.Text{String: body.Nickname, Valid: body.Nickname != ""},
		Bio:               pgtype.Text{String: body.Bio, Valid: body.Bio != ""},
		Country:           pgtype.Text{String: body.Country, Valid: body.Country != ""},
		Age:               pgtype.Int4{Int32: body.Age, Valid: body.Age > 0},
		Timezone:          tz,
		Language:          lang,
		SocialLinks:       socialLinks,
		ProfileVisibility: visibility,
	})
	if err != nil {
		h.log.Error("update profile", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, toProfileResponse(user))
}

// UpdateOnboarding updates the user's onboarding step.
func (h *Handler) UpdateOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, err := handler.UserIDFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	body, err := httputil.DecodeJSON[updateOnboardingRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := h.db.Queries.UpdateUserOnboardingStep(r.Context(), authstore.UpdateUserOnboardingStepParams{
		ID:             userID,
		OnboardingStep: body.Step,
	}); err != nil {
		h.log.Error("update onboarding", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, updateOnboardingResponse{OnboardingStep: body.Step})
}
