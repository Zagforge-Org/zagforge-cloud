package team

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type teamResponse struct {
	ID          string `json:"id"`
	OrgID       string `json:"org_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func toTeamResponse(t authstore.Team) teamResponse {
	r := teamResponse{
		ID:        httputil.UUIDToString(t.ID),
		OrgID:     httputil.UUIDToString(t.OrgID),
		Slug:      t.Slug,
		Name:      t.Name,
		CreatedAt: t.CreatedAt.Time.Format(time.RFC3339),
	}
	if t.Description.Valid {
		r.Description = t.Description.String
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

func toMemberResponse(m authstore.ListTeamMembersRow) memberResponse {
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

type createTeamRequest struct {
	Slug        string `json:"slug" validate:"required,min=2,max=50"`
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}

type updateTeamRequest struct {
	Slug        string `json:"slug" validate:"required,min=2,max=50"`
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}

type addMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required,oneof=lead member"`
}

type updateRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=lead member"`
}
