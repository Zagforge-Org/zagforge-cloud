package main

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/config"
	"github.com/LegationPro/zagforge/auth/internal/db"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	"github.com/LegationPro/zagforge/auth/internal/service/encryption"
	oauthsvc "github.com/LegationPro/zagforge/auth/internal/service/oauth"
	oauthgithub "github.com/LegationPro/zagforge/auth/internal/service/oauth/github"
	oauthgoogle "github.com/LegationPro/zagforge/auth/internal/service/oauth/google"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	"github.com/LegationPro/zagforge/shared/go/dbpool"
)

type deps struct {
	pool       *pgxpool.Pool
	database   *db.DB
	rdb        *redis.Client
	tokenSvc   *token.Service
	encSvc     *encryption.Service
	providers  map[string]oauthsvc.Provider
	sessionSvc *sessionsvc.Service
	auditSvc   *audit.Service
}

func initDeps(ctx context.Context, c *config.Config, log *zap.Logger) (*deps, func(), error) {
	pool, err := dbpool.Connect(ctx, c.DB.URL, dbpool.DefaultConfig(), log)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to db: %w", err)
	}

	database := db.New(pool)

	redisOpts, err := redis.ParseURL(c.Redis.URL)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("connect to redis: %w", err)
	}

	tokenSvc, err := token.New(
		c.App.JWTPrivateKeyBase64,
		c.App.JWTPublicKeyBase64,
		c.App.JWTIssuer,
		c.App.JWTAccessTokenTTL,
		c.App.JWTRefreshTokenTTL,
	)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("init token service: %w", err)
	}

	encKeyBytes, err := base64.StdEncoding.DecodeString(c.App.EncryptionKeyBase64)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("decode encryption key: %w", err)
	}
	encSvc, err := encryption.New(encKeyBytes)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("init encryption: %w", err)
	}

	providers := map[string]oauthsvc.Provider{
		"github": oauthgithub.New(c.App.GithubOAuthClientID, c.App.GithubOAuthClientSecret, c.App.OAuthCallbackBaseURL),
		"google": oauthgoogle.New(c.App.GoogleOAuthClientID, c.App.GoogleOAuthClientSecret, c.App.OAuthCallbackBaseURL),
	}

	sessionSvc := sessionsvc.New(database.Queries, c.App.SessionMaxAge)
	auditSvc := audit.New(database.Queries)

	cleanup := func() {
		if err := rdb.Close(); err != nil {
			log.Warn("failed to close redis", zap.Error(err))
		}
		pool.Close()
	}

	return &deps{
		pool:       pool,
		database:   database,
		rdb:        rdb,
		tokenSvc:   tokenSvc,
		encSvc:     encSvc,
		providers:  providers,
		sessionSvc: sessionSvc,
		auditSvc:   auditSvc,
	}, cleanup, nil
}
