package github

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	oauthgithub "golang.org/x/oauth2/github"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

var _ oauth.Provider = (*Provider)(nil)

// Provider implements the OAuth2 flow for GitHub.
type Provider struct {
	cfg    oauth2.Config
	client *apiClient
}

// New creates a GitHub OAuth2 provider.
func New(clientID, clientSecret, callbackURL string) *Provider {
	return &Provider{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     oauthgithub.Endpoint,
			RedirectURL:  callbackURL + callbackPath,
			Scopes:       defaultScopes,
		},
		client: &apiClient{},
	}
}

func (p *Provider) AuthURL(state string) string {
	return p.cfg.AuthCodeURL(state)
}

func (p *Provider) Exchange(ctx context.Context, code string) (string, string, oauth.UserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", oauth.UserInfo{}, fmt.Errorf("github exchange: %w", err)
	}

	user, err := p.client.fetchUser(ctx, token.AccessToken)
	if err != nil {
		return "", "", oauth.UserInfo{}, err
	}

	return token.AccessToken, "", user, nil
}
