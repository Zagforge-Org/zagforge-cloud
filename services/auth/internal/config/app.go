package config

import "time"

type AppConfig struct {
	JWTPrivateKeyBase64 string        `env:"JWT_PRIVATE_KEY_BASE64,required"`
	JWTPublicKeyBase64  string        `env:"JWT_PUBLIC_KEY_BASE64,required"`
	JWTIssuer           string        `env:"JWT_ISSUER,required"`
	JWTAccessTokenTTL   time.Duration `env:"JWT_ACCESS_TOKEN_TTL" envDefault:"15m"`
	JWTRefreshTokenTTL  time.Duration `env:"JWT_REFRESH_TOKEN_TTL" envDefault:"168h"` // 7 days

	GithubOAuthClientID     string `env:"GITHUB_OAUTH_CLIENT_ID,required"`
	GithubOAuthClientSecret string `env:"GITHUB_OAUTH_CLIENT_SECRET,required"`
	GoogleOAuthClientID     string `env:"GOOGLE_OAUTH_CLIENT_ID,required"`
	GoogleOAuthClientSecret string `env:"GOOGLE_OAUTH_CLIENT_SECRET,required"`
	OAuthCallbackBaseURL    string `env:"OAUTH_CALLBACK_BASE_URL,required"`

	EncryptionKeyBase64 string `env:"ENCRYPTION_KEY_BASE64,required"`

	JWKSKeyID   string `env:"JWKS_KEY_ID" envDefault:"zagforge-auth-1"`
	FrontendURL string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`

	SessionMaxAge time.Duration `env:"SESSION_MAX_AGE" envDefault:"720h"` // 30 days
}
