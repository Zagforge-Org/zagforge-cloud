package github_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/LegationPro/zagforge/shared/go/provider/github"
	"go.uber.org/zap"
)

// generateTestPEM creates a throwaway RSA private key in PEM format for tests.
func generateTestPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// newTestClient creates an APIClient + ClientHandler pointing at the given test server.
func newTestClient(t *testing.T, serverURL string) *gh.ClientHandler {
	t.Helper()
	pem := generateTestPEM(t)
	api, err := gh.NewAPIClient(12345, pem, "webhook-secret",
		gh.WithBaseURL(serverURL),
	)
	if err != nil {
		t.Fatalf("NewAPIClient: %v", err)
	}
	handler, err := gh.NewClientHandler(api, zap.NewNop())
	if err != nil {
		t.Fatalf("NewClientHandler: %v", err)
	}
	return handler
}

func TestGetBlob_Success(t *testing.T) {
	mux := http.NewServeMux()

	// Mock installation token endpoint.
	mux.HandleFunc("POST /app/installations/42/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_test_token"}`)
	})

	// Mock blob endpoint — return raw content.
	mux.HandleFunc("GET /repos/org/repo/git/blobs/abc123", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
			t.Errorf("expected raw accept header, got %q", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") != "token ghs_test_token" {
			t.Errorf("expected installation token auth, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "package main\n\nfunc main() {}")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	content, err := client.GetBlob(context.Background(), 42, "org/repo", "abc123")
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if content != "package main\n\nfunc main() {}" {
		t.Errorf("got %q, want %q", content, "package main\n\nfunc main() {}")
	}
}

func TestGetBlob_NotFound(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /app/installations/42/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"ghs_test_token"}`)
	})

	mux.HandleFunc("GET /repos/org/repo/git/blobs/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.GetBlob(context.Background(), 42, "org/repo", "missing")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetBlob_TokenError(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /app/installations/42/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.GetBlob(context.Background(), 42, "org/repo", "abc123")
	if err == nil {
		t.Fatal("expected error when token generation fails")
	}
}
