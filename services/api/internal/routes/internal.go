package routes

import (
	"fmt"

	"github.com/LegationPro/zagforge/api/internal/middleware/bodylimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/contenttype"
	jobtokenmw "github.com/LegationPro/zagforge/api/internal/middleware/jobtoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerInternal(r *router.Router, d *Deps) error {
	// Webhooks — body limit + Content-Type + rate limited.
	webhooks := r.Group()
	webhooks.Use(bodylimit.Limit(bodyLimit1MB))
	webhooks.Use(contenttype.RequireJSON())
	webhooks.Use(ratelimit.RateLimit(d.RDB, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitWebhook,
		Window:      rateLimitWindow,
	}, "webhook", d.Log))
	if err := webhooks.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/webhooks/github", Handler: d.Webhook.ServeHTTP},
	}); err != nil {
		return fmt.Errorf("register webhook routes: %w", err)
	}

	// Job callbacks — body limit + Content-Type + signed job token auth.
	callbacks := r.Group()
	callbacks.Use(bodylimit.Limit(bodyLimit1MB))
	callbacks.Use(contenttype.RequireJSON())
	callbacks.Use(jobtokenmw.Auth(d.Signer, d.Log))
	if err := callbacks.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/jobs/start", Handler: d.Callback.Start},
		{Method: router.POST, Path: "/internal/jobs/complete", Handler: d.Callback.Complete},
	}); err != nil {
		return fmt.Errorf("register callback routes: %w", err)
	}

	return nil
}
