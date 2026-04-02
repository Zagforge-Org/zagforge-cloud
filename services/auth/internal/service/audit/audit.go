package audit

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
)

// Service records audit events.
type Service struct {
	queries *authstore.Queries
}

// New creates an audit service.
func New(queries *authstore.Queries) *Service {
	return &Service{queries: queries}
}

// LogParams holds data for an audit log entry.
type LogParams struct {
	OrgID      pgtype.UUID
	ActorID    pgtype.UUID
	Action     string
	TargetType string
	TargetID   pgtype.UUID
	Request    *http.Request
	Metadata   []byte // JSON
}

// Log records an audit event.
func (s *Service) Log(ctx context.Context, p LogParams) {
	var ipAddr *netip.Addr
	if p.Request != nil {
		ipStr := extractIP(p.Request)
		if parsed, err := netip.ParseAddr(ipStr); err == nil {
			ipAddr = &parsed
		}
	}

	var ua pgtype.Text
	if p.Request != nil && p.Request.UserAgent() != "" {
		ua = pgtype.Text{String: p.Request.UserAgent(), Valid: true}
	}

	var targetType pgtype.Text
	if p.TargetType != "" {
		targetType = pgtype.Text{String: p.TargetType, Valid: true}
	}

	metadata := p.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	// Best-effort logging — don't fail the request if audit fails.
	_ = s.queries.CreateAuditLog(ctx, authstore.CreateAuditLogParams{
		OrgID:      p.OrgID,
		ActorID:    p.ActorID,
		Action:     p.Action,
		TargetType: targetType,
		TargetID:   p.TargetID,
		IpAddress:  ipAddr,
		UserAgent:  ua,
		Metadata:   metadata,
	})
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
