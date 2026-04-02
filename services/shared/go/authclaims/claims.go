package authclaims

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type contextKey struct{}

var ErrClaimsNotFound = errors.New("auth claims not found in context")

// OrgClaim holds the active organization context from the JWT.
type OrgClaim struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

// Claims are the custom JWT claims issued by the auth service.
type Claims struct {
	jwt.RegisteredClaims
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Org   OrgClaim `json:"org"`
	Teams []string `json:"teams,omitempty"`
	MFA   bool     `json:"mfa"`
}

// NewContext stores claims in the given context.
func NewContext(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext retrieves claims from the context.
func FromContext(ctx context.Context) (*Claims, error) {
	c, ok := ctx.Value(contextKey{}).(*Claims)
	if !ok || c == nil {
		return nil, ErrClaimsNotFound
	}
	return c, nil
}

// OrgUUID parses the org ID from claims into a pgtype.UUID.
func (c *Claims) OrgUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(c.Org.ID); err != nil {
		return id, err
	}
	return id, nil
}

// SubjectUUID parses the subject (user ID) from claims into a pgtype.UUID.
func (c *Claims) SubjectUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(c.Subject); err != nil {
		return id, err
	}
	return id, nil
}
