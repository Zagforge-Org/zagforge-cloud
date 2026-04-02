package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

// apiClient handles HTTP communication with the GitHub API.
type apiClient struct{}

func (c *apiClient) fetchUser(ctx context.Context, accessToken string) (oauth.UserInfo, error) {
	user, err := apiGet[userResponse](ctx, accessToken, userEndpoint)
	if err != nil {
		return oauth.UserInfo{}, fmt.Errorf("github fetch user: %w", err)
	}

	email := user.Email
	if email == "" {
		email, _ = c.fetchPrimaryEmail(ctx, accessToken)
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}

	return oauth.UserInfo{
		ProviderID:  fmt.Sprintf("%d", user.ID),
		Email:       email,
		DisplayName: displayName,
		AvatarURL:   user.AvatarURL,
	}, nil
}

func (c *apiClient) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	emails, err := apiGet[[]emailResponse](ctx, accessToken, emailEndpoint)
	if err != nil {
		return "", fmt.Errorf("github fetch emails: %w", err)
	}
	for _, e := range *emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}

func apiGet[T any](ctx context.Context, token, url string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", acceptHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api %s: %d %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
