package routes

import (
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerGithubAuth(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitOAuth,
		Window:      rateLimitWindow,
	}, "oauth", d.Log))
	return g.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/github/install", Handler: d.GithubAuth.Install},
		{Method: router.GET, Path: "/auth/github/callback", Handler: d.GithubAuth.Callback},
	})
}
