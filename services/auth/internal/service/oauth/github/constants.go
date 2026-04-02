package github

const (
	callbackPath = "/auth/oauth/github/callback"

	apiBaseURL    = "https://api.github.com"
	userEndpoint  = apiBaseURL + "/user"
	emailEndpoint = apiBaseURL + "/user/emails"

	acceptHeader = "application/vnd.github+json"
)

// defaultScopes are the OAuth2 scopes requested from GitHub.
var defaultScopes = []string{
	ScopeReadUser,
	ScopeUserEmail,
}

// GitHub OAuth2 scopes.
const (
	ScopeReadUser  = "read:user"
	ScopeUserEmail = "user:email"
)
