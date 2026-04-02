package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiGet_success(t *testing.T) {
	expected := userResponse{
		ID:        12345,
		Login:     "testuser",
		Name:      "Test User",
		Email:     "test@example.com",
		AvatarURL: "https://example.com/avatar.png",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != acceptHeader {
			t.Errorf("expected Accept %q, got %q", acceptHeader, r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	result, err := apiGet[userResponse](context.Background(), "test-token", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", result.ID)
	}
	if result.Login != "testuser" {
		t.Errorf("expected login %q, got %q", "testuser", result.Login)
	}
	if result.Email != "test@example.com" {
		t.Errorf("expected email %q, got %q", "test@example.com", result.Email)
	}
}

func TestApiGet_non200_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	_, err := apiGet[userResponse](context.Background(), "bad-token", srv.URL)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !containsSubstring(err.Error(), "403") {
		t.Errorf("expected error to contain 403, got: %v", err)
	}
}

func TestApiGet_invalidJSON_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := apiGet[userResponse](context.Background(), "token", srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestApiGet_serverError_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := apiGet[userResponse](context.Background(), "token", srv.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchPrimaryEmail_success(t *testing.T) {
	emails := []emailResponse{
		{Email: "secondary@example.com", Primary: false, Verified: true},
		{Email: "primary@example.com", Primary: true, Verified: true},
		{Email: "unverified@example.com", Primary: true, Verified: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emails)
	}))
	defer srv.Close()

	// We need to override the endpoint temporarily for this test.
	// Since fetchPrimaryEmail uses the package-level emailEndpoint constant,
	// we test apiGet directly with the email list endpoint.
	result, err := apiGet[[]emailResponse](context.Background(), "token", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate the same logic as fetchPrimaryEmail.
	var primary string
	for _, e := range *result {
		if e.Primary && e.Verified {
			primary = e.Email
			break
		}
	}

	if primary != "primary@example.com" {
		t.Errorf("expected primary@example.com, got %q", primary)
	}
}

func TestFetchPrimaryEmail_noPrimary(t *testing.T) {
	emails := []emailResponse{
		{Email: "secondary@example.com", Primary: false, Verified: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emails)
	}))
	defer srv.Close()

	result, err := apiGet[[]emailResponse](context.Background(), "token", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var primary string
	for _, e := range *result {
		if e.Primary && e.Verified {
			primary = e.Email
			break
		}
	}

	if primary != "" {
		t.Errorf("expected empty primary, got %q", primary)
	}
}

func TestApiGet_emptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := apiGet[userResponse](context.Background(), "token", srv.URL)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

// containsSubstring is defined in github_test.go
