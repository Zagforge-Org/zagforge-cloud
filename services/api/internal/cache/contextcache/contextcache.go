package contextcache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const ttl = 10 * time.Minute

// Key returns the Redis key for an assembled context.
// Namespace "ctx:" is separate from rate limiter keys ("rl:").
func Key(repoID, commitSha string) string {
	return fmt.Sprintf("ctx:%s:%s", repoID, commitSha)
}

// Cache is the interface for getting/setting assembled context strings.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}

// RedisCache is the production implementation.
type RedisCache struct{ rdb *redis.Client }

func NewRedis(rdb *redis.Client) *RedisCache {
	return &RedisCache{
		rdb: rdb,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// InMemoryCache is thread-safe, for unit tests only.
type InMemoryCache struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewInMemory() *InMemoryCache { return &InMemoryCache{store: map[string]string{}} }

func (c *InMemoryCache) Get(_ context.Context, key string) (string, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	return v, ok, nil
}

func (c *InMemoryCache) Set(_ context.Context, key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
	return nil
}
