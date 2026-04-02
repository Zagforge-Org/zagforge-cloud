package routes

import (
	"time"

	"github.com/LegationPro/zagforge/auth/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerOAuth(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.Config{
		MaxRequests: 30,
		Window:      1 * time.Minute,
	}, "oauth", d.Log))
	return g.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/oauth/{provider}/start", Handler: d.OAuth.Start},
		{Method: router.GET, Path: "/auth/oauth/{provider}/callback", Handler: d.OAuth.Callback},
	})
}
