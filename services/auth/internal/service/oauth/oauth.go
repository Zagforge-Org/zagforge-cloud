package oauth

import "context"

// UserInfo represents the user data returned by an OAuth provider.
type UserInfo struct {
	ProviderID  string
	Email       string
	DisplayName string
	AvatarURL   string
}

// Provider abstracts an OAuth2 provider.
type Provider interface {
	// AuthURL returns the URL to redirect the user to for authorization.
	AuthURL(state string) string
	// Exchange exchanges an authorization code for an access token and user info.
	Exchange(ctx context.Context, code string) (accessToken string, refreshToken string, user UserInfo, err error)
}
