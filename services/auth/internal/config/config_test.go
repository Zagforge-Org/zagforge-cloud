package config

import (
	"os"
	"testing"
)

var allEnvVars = []string{
	"APP_ENV", "ENV_FILE",
	"JWT_PRIVATE_KEY_BASE64", "JWT_PUBLIC_KEY_BASE64", "JWT_ISSUER",
	"JWT_ACCESS_TOKEN_TTL", "JWT_REFRESH_TOKEN_TTL",
	"GITHUB_OAUTH_CLIENT_ID", "GITHUB_OAUTH_CLIENT_SECRET",
	"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET",
	"OAUTH_CALLBACK_BASE_URL",
	"ENCRYPTION_KEY_BASE64",
	"JWKS_KEY_ID", "FRONTEND_URL", "SESSION_MAX_AGE",
	"PORT",
	"AUTH_DATABASE_URL", "REDIS_URL",
	"CORS_ALLOWED_ORIGINS",
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	originals := make(map[string]string, len(allEnvVars))
	for _, k := range allEnvVars {
		originals[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"JWT_PRIVATE_KEY_BASE64":     "dGVzdC1wcml2YXRl",
		"JWT_PUBLIC_KEY_BASE64":      "dGVzdC1wdWJsaWM=",
		"JWT_ISSUER":                 "http://localhost:8081",
		"GITHUB_OAUTH_CLIENT_ID":     "gh-client-id",
		"GITHUB_OAUTH_CLIENT_SECRET": "gh-client-secret",
		"GOOGLE_OAUTH_CLIENT_ID":     "google-client-id",
		"GOOGLE_OAUTH_CLIENT_SECRET": "google-client-secret",
		"OAUTH_CALLBACK_BASE_URL":    "http://localhost:8081",
		"ENCRYPTION_KEY_BASE64":      "dGVzdC1lbmNyeXB0aW9uLWtleQ==",
		"PORT":                       "8081",
		"AUTH_DATABASE_URL":          "postgres://localhost/test_auth",
		"REDIS_URL":                  "redis://localhost:6379/1",
	}
}

func TestLoad_success(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.JWTIssuer != "http://localhost:8081" {
		t.Errorf("expected issuer %q, got %q", "http://localhost:8081", cfg.App.JWTIssuer)
	}
	if cfg.App.GithubOAuthClientID != "gh-client-id" {
		t.Errorf("expected github client id %q, got %q", "gh-client-id", cfg.App.GithubOAuthClientID)
	}
	if cfg.Server.Port != "8081" {
		t.Errorf("expected port %q, got %q", "8081", cfg.Server.Port)
	}
	if cfg.DB.URL != "postgres://localhost/test_auth" {
		t.Errorf("expected db url %q, got %q", "postgres://localhost/test_auth", cfg.DB.URL)
	}
}

func TestLoad_defaults(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.App.JWTAccessTokenTTL.String() != "15m0s" {
		t.Errorf("expected default access TTL 15m, got %v", cfg.App.JWTAccessTokenTTL)
	}
	if cfg.App.JWTRefreshTokenTTL.String() != "168h0m0s" {
		t.Errorf("expected default refresh TTL 168h, got %v", cfg.App.JWTRefreshTokenTTL)
	}
	if cfg.App.FrontendURL != "http://localhost:3000" {
		t.Errorf("expected default frontend URL %q, got %q", "http://localhost:3000", cfg.App.FrontendURL)
	}
	if cfg.App.JWKSKeyID != "zagforge-auth-1" {
		t.Errorf("expected default JWKS key ID %q, got %q", "zagforge-auth-1", cfg.App.JWKSKeyID)
	}
	if cfg.App.SessionMaxAge.String() != "720h0m0s" {
		t.Errorf("expected default session max age 720h, got %v", cfg.App.SessionMaxAge)
	}
}

func TestLoad_missingRequired(t *testing.T) {
	requiredVars := []string{
		"JWT_PRIVATE_KEY_BASE64",
		"JWT_PUBLIC_KEY_BASE64",
		"JWT_ISSUER",
		"GITHUB_OAUTH_CLIENT_ID",
		"GITHUB_OAUTH_CLIENT_SECRET",
		"GOOGLE_OAUTH_CLIENT_ID",
		"GOOGLE_OAUTH_CLIENT_SECRET",
		"OAUTH_CALLBACK_BASE_URL",
		"ENCRYPTION_KEY_BASE64",
		"PORT",
		"AUTH_DATABASE_URL",
		"REDIS_URL",
	}

	for _, missing := range requiredVars {
		t.Run(missing, func(t *testing.T) {
			env := validEnv()
			delete(env, missing)
			setEnv(t, env)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for missing %s", missing)
			}
		})
	}
}

func TestLoad_customTTL(t *testing.T) {
	env := validEnv()
	env["JWT_ACCESS_TOKEN_TTL"] = "30m"
	env["JWT_REFRESH_TOKEN_TTL"] = "336h"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.JWTAccessTokenTTL.String() != "30m0s" {
		t.Errorf("expected 30m, got %v", cfg.App.JWTAccessTokenTTL)
	}
	if cfg.App.JWTRefreshTokenTTL.String() != "336h0m0s" {
		t.Errorf("expected 336h, got %v", cfg.App.JWTRefreshTokenTTL)
	}
}
