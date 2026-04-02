package routes

import (
	"time"

	"github.com/LegationPro/zagforge/auth/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerSession(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.Config{
		MaxRequests: 30,
		Window:      1 * time.Minute,
	}, "refresh", d.Log))
	return g.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/token/refresh", Handler: d.Session.Refresh},
		{Method: router.POST, Path: "/auth/logout", Handler: d.Session.Logout},
	})
}
