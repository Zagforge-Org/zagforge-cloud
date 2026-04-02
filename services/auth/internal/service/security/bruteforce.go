package security

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
)

const maxFailedAttempts = 10

// Service handles brute-force detection.
type Service struct {
	queries *authstore.Queries
}

// New creates a security service.
func New(queries *authstore.Queries) *Service {
	return &Service{queries: queries}
}

// IsBlocked returns true if the identifier (email) or IP has exceeded the
// failed login attempt threshold within the time window.
func (s *Service) IsBlocked(ctx context.Context, identifier string, r *http.Request) (bool, error) {
	count, err := s.queries.CountRecentFailedLogins(ctx, identifier)
	if err != nil {
		return false, err
	}
	if count >= maxFailedAttempts {
		return true, nil
	}

	ip := parseIP(r)
	ipCount, err := s.queries.CountRecentFailedLoginsByIP(ctx, ip)
	if err != nil {
		return false, err
	}
	return ipCount >= maxFailedAttempts*3, nil
}

// RecordFailure records a failed login attempt.
func (s *Service) RecordFailure(ctx context.Context, identifier string, r *http.Request) error {
	ip := parseIP(r)
	return s.queries.RecordFailedLogin(ctx, authstore.RecordFailedLoginParams{
		Identifier: identifier,
		IpAddress:  ip,
		UserAgent:  pgtype.Text{String: r.UserAgent(), Valid: true},
	})
}

func parseIP(r *http.Request) netip.Addr {
	ipStr := extractIP(r)
	addr, _ := netip.ParseAddr(ipStr)
	return addr
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
