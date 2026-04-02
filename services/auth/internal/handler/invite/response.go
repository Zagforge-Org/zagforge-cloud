package invite

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type inviteResponse struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

func toInviteResponse(i authstore.Invite) inviteResponse {
	return inviteResponse{
		ID:        httputil.UUIDToString(i.ID),
		OrgID:     httputil.UUIDToString(i.OrgID),
		Email:     i.Email,
		Role:      i.Role,
		Status:    i.Status,
		ExpiresAt: i.ExpiresAt.Time.Format(time.RFC3339),
		CreatedAt: i.CreatedAt.Time.Format(time.RFC3339),
	}
}

type createInviteRequest struct {
	Email string `json:"email" validate:"required,email,max=255"`
	Role  string `json:"role" validate:"required,oneof=admin member"`
}

type acceptInviteRequest struct {
	Token string `json:"token" validate:"required"`
}
