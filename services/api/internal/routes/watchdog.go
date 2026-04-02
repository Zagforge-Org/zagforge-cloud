package routes

import (
	"github.com/LegationPro/zagforge/api/internal/middleware/watchdogauth"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerWatchdog(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(watchdogauth.SharedSecret(d.WatchdogSecret))
	return g.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/watchdog/timeout", Handler: d.Watchdog.Timeout},
	})
}
