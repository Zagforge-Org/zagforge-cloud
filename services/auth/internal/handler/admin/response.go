package admin

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type userResponse struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	OnboardingStep  string `json:"onboarding_step"`
	CreatedAt       string `json:"created_at"`
}

func toUserResponse(u authstore.User) userResponse {
	r := userResponse{
		ID:              httputil.UUIDToString(u.ID),
		Email:           u.Email,
		IsPlatformAdmin: u.IsPlatformAdmin,
		OnboardingStep:  u.OnboardingStep,
		CreatedAt:       u.CreatedAt.Time.Format(time.RFC3339),
	}
	if u.FirstName.Valid {
		r.FirstName = u.FirstName.String
	}
	if u.LastName.Valid {
		r.LastName = u.LastName.String
	}
	return r
}

type orgResponse struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Plan       string `json:"plan"`
	MaxMembers int32  `json:"max_members"`
	CreatedAt  string `json:"created_at"`
}

func toOrgResponse(o authstore.Organization) orgResponse {
	return orgResponse{
		ID:         httputil.UUIDToString(o.ID),
		Slug:       o.Slug,
		Name:       o.Name,
		Plan:       o.Plan,
		MaxMembers: o.MaxMembers,
		CreatedAt:  o.CreatedAt.Time.Format(time.RFC3339),
	}
}

type updateUserRequest struct {
	IsPlatformAdmin *bool `json:"is_platform_admin"`
}

type updateOrgPlanRequest struct {
	Plan       string `json:"plan" validate:"required,oneof=free pro enterprise"`
	MaxMembers int32  `json:"max_members" validate:"required,gte=1"`
}

type listResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}
