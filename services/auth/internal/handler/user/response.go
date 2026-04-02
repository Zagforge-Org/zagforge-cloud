package user

import (
	"encoding/json"
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type profileResponse struct {
	ID                string          `json:"id"`
	Email             string          `json:"email"`
	EmailVerified     bool            `json:"email_verified"`
	FirstName         string          `json:"first_name,omitempty"`
	LastName          string          `json:"last_name,omitempty"`
	Nickname          string          `json:"nickname,omitempty"`
	Bio               string          `json:"bio,omitempty"`
	Country           string          `json:"country,omitempty"`
	Age               int32           `json:"age,omitempty"`
	Timezone          string          `json:"timezone"`
	Language          string          `json:"language"`
	AvatarURL         string          `json:"avatar_url,omitempty"`
	SocialLinks       json.RawMessage `json:"social_links"`
	ProfileVisibility string          `json:"profile_visibility"`
	OnboardingStep    string          `json:"onboarding_step"`
	CreatedAt         string          `json:"created_at"`
}

func toProfileResponse(u authstore.User) profileResponse {
	r := profileResponse{
		ID:                httputil.UUIDToString(u.ID),
		Email:             u.Email,
		EmailVerified:     u.EmailVerified,
		Timezone:          u.Timezone,
		Language:          u.Language,
		SocialLinks:       u.SocialLinks,
		ProfileVisibility: u.ProfileVisibility,
		OnboardingStep:    u.OnboardingStep,
		CreatedAt:         u.CreatedAt.Time.Format(time.RFC3339),
	}
	if u.FirstName.Valid {
		r.FirstName = u.FirstName.String
	}
	if u.LastName.Valid {
		r.LastName = u.LastName.String
	}
	if u.Nickname.Valid {
		r.Nickname = u.Nickname.String
	}
	if u.Bio.Valid {
		r.Bio = u.Bio.String
	}
	if u.Country.Valid {
		r.Country = u.Country.String
	}
	if u.Age.Valid {
		r.Age = u.Age.Int32
	}
	if u.AvatarUrl.Valid {
		r.AvatarURL = u.AvatarUrl.String
	}
	return r
}

type updateProfileRequest struct {
	FirstName         string          `json:"first_name" validate:"max=100"`
	LastName          string          `json:"last_name" validate:"max=100"`
	Nickname          string          `json:"nickname" validate:"max=50"`
	Bio               string          `json:"bio" validate:"max=500"`
	Country           string          `json:"country" validate:"max=100"`
	Age               int32           `json:"age" validate:"omitempty,gte=13,max=150"`
	Timezone          string          `json:"timezone" validate:"max=50"`
	Language          string          `json:"language" validate:"max=10"`
	SocialLinks       json.RawMessage `json:"social_links"`
	ProfileVisibility string          `json:"profile_visibility" validate:"omitempty,oneof=public private team_only"`
}

type updateOnboardingRequest struct {
	Step string `json:"step" validate:"required,oneof=profile connect_oauth join_team completed"`
}

type updateOnboardingResponse struct {
	OnboardingStep string `json:"onboarding_step"`
}

type identityResponse struct {
	Provider    string `json:"provider"`
	ProviderID  string `json:"provider_id"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}
