package config

type AppConfig struct {
	GithubAppID            int64  `env:"GITHUB_APP_ID,required"`
	GithubAppSlug          string `env:"GITHUB_APP_SLUG,required"`
	GithubAppPrivateKey    string `env:"GITHUB_APP_PRIVATE_KEY,required"`
	GithubAppWebhookSecret string `env:"GITHUB_APP_WEBHOOK_SECRET,required"`
	JWTPublicKeyBase64     string `env:"JWT_PUBLIC_KEY_BASE64,required"`
	JWTIssuer              string `env:"JWT_ISSUER,required"`
	HMACSigningKey         string `env:"HMAC_SIGNING_KEY,required"`
	HMACSigningKeyPrev     string `env:"HMAC_SIGNING_KEY_PREV"` // previous key for rotation grace period
	WatchdogSecret         string `env:"WATCHDOG_SECRET,required"`
	EncryptionKeyBase64    string `env:"ENCRYPTION_KEY_BASE64,required"`
	CLIAPIKey              string `env:"CLI_API_KEY,required"`
}
