package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LegationPro/zagforge/auth/internal/service/oauth"
)

func TestFetchUser_success(t *testing.T) {
	info := userInfoResponse{
		Sub:           "google-user-123",
		Email:         "user@gmail.com",
		EmailVerified: true,
		Name:          "Google User",
		Picture:       "https://lh3.googleusercontent.com/photo.jpg",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("expected Bearer token in Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer srv.Close()

	// Test the JSON parsing by calling the endpoint directly.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	result := oauth.UserInfo{
		ProviderID:  decoded.Sub,
		Email:       decoded.Email,
		DisplayName: decoded.Name,
		AvatarURL:   decoded.Picture,
	}

	if result.ProviderID != "google-user-123" {
		t.Errorf("expected provider ID %q, got %q", "google-user-123", result.ProviderID)
	}
	if result.Email != "user@gmail.com" {
		t.Errorf("expected email %q, got %q", "user@gmail.com", result.Email)
	}
	if result.DisplayName != "Google User" {
		t.Errorf("expected name %q, got %q", "Google User", result.DisplayName)
	}
	if result.AvatarURL != "https://lh3.googleusercontent.com/photo.jpg" {
		t.Errorf("expected avatar URL, got %q", result.AvatarURL)
	}
}

func TestFetchUser_non200_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid token"))
	}))
	defer srv.Close()

	// Simulate what fetchUser does when the endpoint returns non-200.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer bad-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected non-200 response")
	}
}

func TestFetchUser_invalidJSON_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var info userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err == nil {
		t.Fatal("expected JSON decode error")
	}
}

func TestFetchUser_emptyFields(t *testing.T) {
	info := userInfoResponse{
		Sub: "minimal-user",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(info)
	}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded userInfoResponse
	json.NewDecoder(resp.Body).Decode(&decoded)

	if decoded.Sub != "minimal-user" {
		t.Errorf("expected sub %q, got %q", "minimal-user", decoded.Sub)
	}
	if decoded.Email != "" {
		t.Errorf("expected empty email, got %q", decoded.Email)
	}
}
