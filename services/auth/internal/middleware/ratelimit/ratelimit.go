package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

var errRateLimitExceeded = errors.New("rate limit exceeded")

// Config holds the sliding window parameters.
type Config struct {
	MaxRequests int
	Window      time.Duration
}

// keyFunc extracts the rate limit key from a request.
// Authenticated requests use the user ID; unauthenticated use the client IP.
func keyFunc(r *http.Request, prefix string) string {
	if claims, err := authclaims.FromContext(r.Context()); err == nil {
		return fmt.Sprintf("rl:%s:user:%s", prefix, claims.Subject)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return fmt.Sprintf("rl:%s:ip:%s", prefix, ip)
}

// RateLimit returns middleware that enforces sliding window rate limiting via Redis.
func RateLimit(rdb *redis.Client, cfg Config, prefix string, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r, prefix)

			allowed, remaining, err := checkRateLimit(r.Context(), rdb, key, cfg)
			if err != nil {
				log.Error("rate limit check failed, allowing request", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.MaxRequests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				log.Warn("rate limit exceeded", zap.String("key", key))
				retryAfter := int(cfg.Window.Seconds())
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				httputil.ErrResponse(w, http.StatusTooManyRequests, errRateLimitExceeded)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func checkRateLimit(ctx context.Context, rdb *redis.Client, key string, cfg Config) (bool, int, error) {
	now := time.Now()
	windowStart := now.Add(-cfg.Window).UnixMicro()
	nowMicro := now.UnixMicro()
	member := strconv.FormatInt(nowMicro, 10)

	pipe := rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(windowStart, 10))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(nowMicro), Member: member})
	pipe.Expire(ctx, key, cfg.Window+time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, fmt.Errorf("redis pipeline: %w", err)
	}

	count := int(countCmd.Val())
	remaining := max(cfg.MaxRequests-count-1, 0)

	if count >= cfg.MaxRequests {
		return false, 0, nil
	}

	return true, remaining, nil
}
