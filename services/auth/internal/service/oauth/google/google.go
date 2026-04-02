package google

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	oauthgoogle "golang.org/x/oauth2/google"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

var _ oauth.Provider = (*Provider)(nil)

// Provider implements the OAuth2 flow for Google.
type Provider struct {
	cfg    oauth2.Config
	client *apiClient
}

// New creates a Google OAuth2 provider.
func New(clientID, clientSecret, callbackURL string) *Provider {
	return &Provider{
		cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     oauthgoogle.Endpoint,
			RedirectURL:  callbackURL + callbackPath,
			Scopes:       defaultScopes,
		},
		client: &apiClient{},
	}
}

func (p *Provider) AuthURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (p *Provider) Exchange(ctx context.Context, code string) (string, string, oauth.UserInfo, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", oauth.UserInfo{}, fmt.Errorf("google exchange: %w", err)
	}

	user, err := p.client.fetchUser(ctx, token.AccessToken)
	if err != nil {
		return "", "", oauth.UserInfo{}, err
	}

	return token.AccessToken, token.RefreshToken, user, nil
}
