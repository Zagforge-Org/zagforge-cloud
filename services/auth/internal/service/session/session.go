package session

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
)

// Service manages session lifecycle.
type Service struct {
	queries *authstore.Queries
	maxAge  time.Duration
}

// New creates a session service.
func New(queries *authstore.Queries, maxAge time.Duration) *Service {
	return &Service{queries: queries, maxAge: maxAge}
}

// CreateParams holds the data for creating a new session.
type CreateParams struct {
	UserID            pgtype.UUID
	Request           *http.Request
	DeviceFingerprint string
}

// Create creates a new session from request metadata.
func (s *Service) Create(ctx context.Context, p CreateParams) (authstore.Session, error) {
	ipStr := extractIP(p.Request)
	var ipAddr *netip.Addr
	if ipStr != "" {
		if parsed, err := netip.ParseAddr(ipStr); err == nil {
			ipAddr = &parsed
		}
	}

	ua := p.Request.UserAgent()
	var uaText pgtype.Text
	if ua != "" {
		uaText = pgtype.Text{String: ua, Valid: true}
	}

	device := Device("").Parse(ua)
	var deviceNameText pgtype.Text
	if device != DeviceUnknown {
		deviceNameText = pgtype.Text{String: device.String(), Valid: true}
	}

	var fingerprintText pgtype.Text
	if p.DeviceFingerprint != "" {
		fingerprintText = pgtype.Text{String: p.DeviceFingerprint, Valid: true}
	}

	session, err := s.queries.CreateSession(ctx, authstore.CreateSessionParams{
		UserID:            p.UserID,
		IpAddress:         ipAddr,
		UserAgent:         uaText,
		DeviceName:        deviceNameText,
		DeviceFingerprint: fingerprintText,
		Country:           pgtype.Text{},
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(s.maxAge), Valid: true},
	})
	if err != nil {
		return authstore.Session{}, fmt.Errorf("create session: %w", err)
	}

	return session, nil
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
