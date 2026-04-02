package audit

import (
	"time"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type auditLogResponse struct {
	ID         string `json:"id"`
	ActorID    string `json:"actor_id,omitempty"`
	Action     string `json:"action"`
	TargetType string `json:"target_type,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
	IPAddress  string `json:"ip_address,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func toAuditLogResponse(a authstore.AuditLog) auditLogResponse {
	r := auditLogResponse{
		ID:        httputil.UUIDToString(a.ID),
		Action:    a.Action,
		CreatedAt: a.CreatedAt.Time.Format(time.RFC3339),
	}
	if a.ActorID.Valid {
		r.ActorID = httputil.UUIDToString(a.ActorID)
	}
	if a.TargetType.Valid {
		r.TargetType = a.TargetType.String
	}
	if a.TargetID.Valid {
		r.TargetID = httputil.UUIDToString(a.TargetID)
	}
	if a.IpAddress != nil {
		r.IPAddress = a.IpAddress.String()
	}
	return r
}

type loginMetricRow struct {
	Day   string `json:"day"`
	Total int64  `json:"total"`
}
