package config

import (
	"os"
	"strconv"
	"strings"
)

type AppConfig struct {
	GithubAppID            int64
	GithubAppPrivateKey    string
	GithubAppWebhookSecret string
}

func LoadAppConfig() (*AppConfig, error) {
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, notSetErr("GITHUB_APP_ID")
	}

	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, notSetErr("GITHUB_APP_ID")
	}

	webhookSecret := os.Getenv("GITHUB_APP_WEBHOOK_SECRET")
	if webhookSecret == "" {
		return nil, notSetErr("GITHUB_APP_WEBHOOK_SECRET")
	}

	privateKeyStr := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyStr == "" {
		return nil, notSetErr("GITHUB_APP_PRIVATE_KEY")
	}
	// Env vars often store PEM keys with literal \n instead of real newlines.
	privateKeyStr = strings.ReplaceAll(privateKeyStr, `\n`, "\n")

	return &AppConfig{
		GithubAppID:            appID,
		GithubAppPrivateKey:    privateKeyStr,
		GithubAppWebhookSecret: webhookSecret,
	}, nil
}
