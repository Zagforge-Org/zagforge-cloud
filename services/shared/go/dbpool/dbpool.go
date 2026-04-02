package dbpool

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Config holds connection pool tuning parameters.
type Config struct {
	MaxConns          int32         // Maximum open connections (default: 25).
	MinConns          int32         // Minimum idle connections kept warm (default: 5).
	MaxConnLifetime   time.Duration // Maximum lifetime of a connection (default: 1h).
	MaxConnIdleTime   time.Duration // Maximum idle time before a connection is closed (default: 30m).
	HealthCheckPeriod time.Duration // How often idle connections are health-checked (default: 1m).
}

// DefaultConfig returns production-ready pool defaults.
func DefaultConfig() Config {
	return Config{
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   1 * time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
}

// Connect creates a pgxpool.Pool with the given URL and pool configuration.
// The caller is responsible for calling pool.Close() when done.
func Connect(ctx context.Context, url string, poolCfg Config, log *zap.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	cfg.MaxConns = poolCfg.MaxConns
	cfg.MinConns = poolCfg.MinConns
	cfg.MaxConnLifetime = poolCfg.MaxConnLifetime
	cfg.MaxConnIdleTime = poolCfg.MaxConnIdleTime
	cfg.HealthCheckPeriod = poolCfg.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("database pool connected",
		zap.Int32("max_conns", poolCfg.MaxConns),
		zap.Int32("min_conns", poolCfg.MinConns),
		zap.Duration("max_conn_lifetime", poolCfg.MaxConnLifetime),
		zap.Duration("max_conn_idle_time", poolCfg.MaxConnIdleTime),
	)

	return pool, nil
}
