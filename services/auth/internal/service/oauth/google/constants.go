package google

const (
	callbackPath     = "/auth/oauth/google/callback"
	userInfoEndpoint = "https://www.googleapis.com/oauth2/v3/userinfo"
)

// defaultScopes are the OAuth2 scopes requested from Google.
var defaultScopes = []string{
	ScopeOpenID,
	ScopeEmail,
	ScopeProfile,
}

// Google OAuth2 scopes.
const (
	ScopeOpenID  = "openid"
	ScopeEmail   = "email"
	ScopeProfile = "profile"
)
