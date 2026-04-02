package github

import (
	"errors"
	"net/http"
	"time"
)

var (
	defaultAPIBaseURL = "https://api.github.com"
)

// APIClientOption is a functional option for NewAPIClient.
type APIClientOption func(*APIClient)

// WithHTTPClient overrides the HTTP client used for GitHub API calls.
func WithHTTPClient(c *http.Client) APIClientOption {
	return func(a *APIClient) { a.httpClient = c }
}

// WithBaseURL overrides the GitHub API base URL (for testing).
func WithBaseURL(url string) APIClientOption {
	return func(a *APIClient) { a.apiBaseURL = url }
}

// APIClient holds provider credentials. Construct with NewAPIClient.
type APIClient struct {
	appID         int64
	privateKey    []byte
	webhookSecret string
	httpClient    *http.Client
	apiBaseURL    string
}

// NewAPIClient returns a configured APIClient. Returns an error if privateKey or
// webhookSecret is empty — both are required for correct operation at startup.
func NewAPIClient(appID int64, privateKey []byte, webhookSecret string, opts ...APIClientOption) (*APIClient, error) {
	if len(privateKey) == 0 {
		return nil, errors.New("privateKey must not be empty")
	}
	if webhookSecret == "" {
		return nil, errors.New("webhookSecret must not be empty")
	}
	a := &APIClient{
		appID:         appID,
		privateKey:    privateKey,
		webhookSecret: webhookSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		apiBaseURL:    defaultAPIBaseURL,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}
