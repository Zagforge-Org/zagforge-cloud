package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	"github.com/LegationPro/zagforge/api/internal/config"
	"github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/engine"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/shared/go/dbpool"
	"github.com/LegationPro/zagforge/shared/go/jobtoken"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	storagepkg "github.com/LegationPro/zagforge/shared/go/storage"
)

type deps struct {
	pool      *pgxpool.Pool
	database  *db.DB
	rdb       *redis.Client
	ch        *githubprovider.ClientHandler
	jwtPubKey ed25519.PublicKey
	signer    *jobtoken.Signer
	enqueuer  engine.TaskEnqueuer
	gcsClient *storagepkg.Client
	encSvc    *encryption.Service
	ctxCache  *contextcache.RedisCache
}

func initDeps(ctx context.Context, c *config.Config, log *zap.Logger) (*deps, func(), error) {
	pool, err := dbpool.Connect(ctx, c.DB.URL, dbpool.DefaultConfig(), log)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to db: %w", err)
	}

	database := db.New(pool)

	// Redis.
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

	// GitHub App client.
	client, err := githubprovider.NewAPIClient(c.App.GithubAppID, []byte(c.App.GithubAppPrivateKey), c.App.GithubAppWebhookSecret)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("create API client: %w", err)
	}
	ch, err := githubprovider.NewClientHandler(client, log)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("create client handler: %w", err)
	}

	// JWT public key for auth middleware.
	pubKeyPEM, err := base64.StdEncoding.DecodeString(c.App.JWTPublicKeyBase64)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("decode jwt public key: %w", err)
	}
	pubKeyRaw, err := jwt.ParseEdPublicKeyFromPEM(pubKeyPEM)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("parse jwt public key: %w", err)
	}
	jwtPubKey, ok := pubKeyRaw.(ed25519.PublicKey)
	if !ok {
		pool.Close()
		return nil, nil, fmt.Errorf("jwt public key is not Ed25519")
	}

	// HMAC signer for job tokens.
	signer := jobtoken.NewSigner([]byte(c.App.HMACSigningKey), 30*time.Minute)
	if c.App.HMACSigningKeyPrev != "" {
		signer = signer.WithPreviousKey([]byte(c.App.HMACSigningKeyPrev))
		log.Info("HMAC key rotation active: accepting both current and previous signing keys")
	}

	// Cloud Tasks enqueuer (or noop for local dev).
	var enqueuer engine.TaskEnqueuer
	var ctCloser func() error
	if c.CloudTasks.Enabled() {
		ct, err := engine.NewCloudTasksEnqueuer(ctx, engine.CloudTasksConfig{
			Project:        c.CloudTasks.Project,
			Location:       c.CloudTasks.Location,
			Queue:          c.CloudTasks.Queue,
			WorkerURL:      c.CloudTasks.WorkerURL,
			ServiceAccount: c.CloudTasks.ServiceAccount,
		})
		if err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("create cloud tasks enqueuer: %w", err)
		}
		ctCloser = ct.Close
		enqueuer = ct
		log.Info("cloud tasks enqueuer enabled",
			zap.String("queue", c.CloudTasks.Queue),
			zap.String("worker_url", c.CloudTasks.WorkerURL),
		)
	} else {
		enqueuer = engine.NewNoopEnqueuer(log)
		log.Info("cloud tasks not configured, using noop enqueuer (poller mode)")
	}

	// GCS client for snapshot storage.
	gcsClient, err := storagepkg.NewClient(ctx, storagepkg.Config{
		Bucket:   c.GCS.Bucket,
		Endpoint: c.GCS.Endpoint,
	}, log)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("create gcs client: %w", err)
	}

	// Encryption service for AI provider keys.
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

	ctxCache := contextcache.NewRedis(rdb)

	cleanup := func() {
		if ctCloser != nil {
			_ = ctCloser()
		}
		if err := rdb.Close(); err != nil {
			log.Warn("failed to close redis", zap.Error(err))
		}
		pool.Close()
	}

	return &deps{
		pool:      pool,
		database:  database,
		rdb:       rdb,
		ch:        ch,
		jwtPubKey: jwtPubKey,
		signer:    signer,
		enqueuer:  enqueuer,
		gcsClient: gcsClient,
		encSvc:    encSvc,
		ctxCache:  ctxCache,
	}, cleanup, nil
}
