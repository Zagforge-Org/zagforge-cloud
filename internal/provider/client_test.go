package provider_test

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LegationPro/zagforge-mvp-impl/internal/provider"
)

func validClient(t *testing.T) *provider.ClientHandler {
	t.Helper()
	client, err := provider.NewAPIClient(1, []byte("private-key"), "webhook-secret")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	return provider.NewClientHandler(client)
}

// generateRSAKey returns a PEM-encoded RSA private key for use in tests.
func generateRSAKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// rsaClient creates a ClientHandler backed by a real RSA key, pointing at baseURL.
func rsaClient(t *testing.T, baseURL string) *provider.ClientHandler {
	t.Helper()
	apiClient, err := provider.NewAPIClient(
		42,
		generateRSAKey(t),
		"webhook-secret",
		provider.WithBaseURL(baseURL),
	)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	return provider.NewClientHandler(apiClient)
}

func makeSignature(t *testing.T, secret string, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestNewAPIClient_rejectsEmptyWebhookSecret(t *testing.T) {
	_, err := provider.NewAPIClient(1, []byte("key"), "")
	if err == nil {
		t.Fatal("expected error for empty webhookSecret, got nil")
	}
}

func TestNewAPIClient_rejectsEmptyPrivateKey(t *testing.T) {
	_, err := provider.NewAPIClient(1, nil, "secret")
	if err == nil {
		t.Fatal("expected error for nil privateKey, got nil")
	}
}

func TestNewAPIClient_rejectsEmptyPrivateKeySlice(t *testing.T) {
	_, err := provider.NewAPIClient(1, []byte{}, "secret")
	if err == nil {
		t.Fatal("expected error for empty privateKey slice, got nil")
	}
}

func TestNewAPIClient_succeedsWithValidInputs(t *testing.T) {
	_, err := provider.NewAPIClient(1, []byte("key"), "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWebhook_validSignature(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123","repository":{"id":1,"full_name":"org/repo"}}`)
	sig := makeSignature(t, "webhook-secret", payload)

	_, err := ch.ValidateWebhook(context.Background(), payload, sig, "push")
	if err != nil {
		t.Fatalf("expected no error for valid signature, got: %v", err)
	}
}

func TestValidateWebhook_invalidSignature(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`{"ref":"refs/heads/main"}`)

	_, err := ch.ValidateWebhook(context.Background(), payload, "sha256=badhex", "push")
	if !errors.Is(err, provider.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got: %v", err)
	}
}

func TestValidateWebhook_wrongSecret(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`{"ref":"refs/heads/main"}`)
	sig := makeSignature(t, "wrong-secret", payload)

	_, err := ch.ValidateWebhook(context.Background(), payload, sig, "push")
	if !errors.Is(err, provider.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for wrong secret, got: %v", err)
	}
}

func TestValidateWebhook_emptySignature(t *testing.T) {
	ch := validClient(t)

	_, err := ch.ValidateWebhook(context.Background(), []byte("payload"), "", "push")
	if !errors.Is(err, provider.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature for empty signature, got: %v", err)
	}
}

func TestValidateWebhook_populatesEvent(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`{
		"ref": "refs/heads/main",
		"after": "deadbeef",
		"repository": {"id": 42, "full_name": "org/repo"}
	}`)
	sig := makeSignature(t, "webhook-secret", payload)

	event, err := ch.ValidateWebhook(context.Background(), payload, sig, "push")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "push" {
		t.Errorf("expected EventType %q, got %q", "push", event.EventType)
	}
	if event.Branch != "main" {
		t.Errorf("expected Branch %q, got %q", "main", event.Branch)
	}
	if event.CommitSHA != "deadbeef" {
		t.Errorf("expected CommitSHA %q, got %q", "deadbeef", event.CommitSHA)
	}
	if event.RepoID != 42 {
		t.Errorf("expected RepoID %d, got %d", 42, event.RepoID)
	}
	if event.RepoName != "org/repo" {
		t.Errorf("expected RepoName %q, got %q", "org/repo", event.RepoName)
	}
}

func TestValidateWebhook_invalidJSON(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`not-json`)
	sig := makeSignature(t, "webhook-secret", payload)

	_, err := ch.ValidateWebhook(context.Background(), payload, sig, "push")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if errors.Is(err, provider.ErrInvalidSignature) {
		t.Fatal("expected parse error, not ErrInvalidSignature")
	}
}

func TestValidateWebhook_stripsRefPrefix(t *testing.T) {
	ch := validClient(t)
	payload := []byte(`{"ref":"refs/heads/feature/my-branch","after":"abc","repository":{"id":1,"full_name":"org/repo"}}`)
	sig := makeSignature(t, "webhook-secret", payload)

	event, err := ch.ValidateWebhook(context.Background(), payload, sig, "push")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Branch != "feature/my-branch" {
		t.Errorf("expected Branch %q, got %q", "feature/my-branch", event.Branch)
	}
}

func TestGenerateCloneToken_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/app/installations/99/access_tokens" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"token": "ghs_test_token"})
	}))
	defer srv.Close()

	ch := rsaClient(t, srv.URL)
	token, err := ch.GenerateCloneToken(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "ghs_test_token" {
		t.Errorf("expected token %q, got %q", "ghs_test_token", token)
	}
}

func TestGenerateCloneToken_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	}))
	defer srv.Close()

	ch := rsaClient(t, srv.URL)
	_, err := ch.GenerateCloneToken(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error for non-201 response, got nil")
	}
}

func TestGenerateCloneToken_emptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"token": ""})
	}))
	defer srv.Close()

	ch := rsaClient(t, srv.URL)
	_, err := ch.GenerateCloneToken(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestGenerateCloneToken_invalidPrivateKey(t *testing.T) {
	apiClient, err := provider.NewAPIClient(1, []byte("not-a-pem-key"), "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ch := provider.NewClientHandler(apiClient)
	_, err = ch.GenerateCloneToken(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error for invalid private key, got nil")
	}
}

// runGit runs a git command in dir, setting minimal identity env vars.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeBareRepo creates a bare git repo with one empty commit on `main`.
func makeBareRepo(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	runGit(t, src, "init", "-b", "main")
	runGit(t, src, "commit", "--allow-empty", "-m", "init")
	return src
}

func TestCloneRepo_success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	src := makeBareRepo(t)
	dst := filepath.Join(t.TempDir(), "clone")

	ch := validClient(t)
	err := ch.CloneRepo(context.Background(), "file://"+src, "main", "", dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); os.IsNotExist(err) {
		t.Error("expected .git directory in clone destination")
	}
}

func TestCloneRepo_invalidURL(t *testing.T) {
	ch := validClient(t)
	err := ch.CloneRepo(context.Background(), "://bad-url", "main", "", t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestCloneRepo_branchNotFound(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	src := makeBareRepo(t)
	dst := filepath.Join(t.TempDir(), "clone")

	ch := validClient(t)
	err := ch.CloneRepo(context.Background(), "file://"+src, "nonexistent-branch", "", dst)
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
}
