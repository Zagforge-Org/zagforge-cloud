package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

// apiClient handles HTTP communication with the Google API.
type apiClient struct{}

func (c *apiClient) fetchUser(ctx context.Context, accessToken string) (oauth.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return oauth.UserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauth.UserInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return oauth.UserInfo{}, fmt.Errorf("google userinfo: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return oauth.UserInfo{}, err
	}

	return oauth.UserInfo{
		ProviderID:  info.Sub,
		Email:       info.Email,
		DisplayName: info.Name,
		AvatarURL:   info.Picture,
	}, nil
}
