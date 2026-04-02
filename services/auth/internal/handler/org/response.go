package org

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type orgResponse struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	LogoURL      string `json:"logo_url,omitempty"`
	Plan         string `json:"plan"`
	BillingEmail string `json:"billing_email,omitempty"`
	MaxMembers   int32  `json:"max_members"`
	CreatedAt    string `json:"created_at"`
}

func toOrgResponse(o authstore.Organization) orgResponse {
	r := orgResponse{
		ID:         httputil.UUIDToString(o.ID),
		Slug:       o.Slug,
		Name:       o.Name,
		Plan:       o.Plan,
		MaxMembers: o.MaxMembers,
		CreatedAt:  o.CreatedAt.Time.Format(time.RFC3339),
	}
	if o.LogoUrl.Valid {
		r.LogoURL = o.LogoUrl.String
	}
	if o.BillingEmail.Valid {
		r.BillingEmail = o.BillingEmail.String
	}
	return r
}

type memberResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role"`
	JoinedAt  string `json:"joined_at"`
}

func toMemberResponse(m authstore.ListOrgMembersRow) memberResponse {
	r := memberResponse{
		ID:       httputil.UUIDToString(m.ID),
		UserID:   httputil.UUIDToString(m.UserID),
		Email:    m.Email,
		Role:     m.Role,
		JoinedAt: m.CreatedAt.Time.Format(time.RFC3339),
	}
	if m.FirstName.Valid {
		r.FirstName = m.FirstName.String
	}
	if m.LastName.Valid {
		r.LastName = m.LastName.String
	}
	if m.AvatarUrl.Valid {
		r.AvatarURL = m.AvatarUrl.String
	}
	return r
}

type createOrgRequest struct {
	Slug string `json:"slug" validate:"required,min=2,max=50"`
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type updateOrgRequest struct {
	Name         string `json:"name" validate:"required,min=1,max=100"`
	LogoURL      string `json:"logo_url" validate:"max=500"`
	BillingEmail string `json:"billing_email" validate:"omitempty,email,max=255"`
}

type updateRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin member"`
}

type transferRequest struct {
	NewOwnerID string `json:"new_owner_id" validate:"required,uuid"`
}
