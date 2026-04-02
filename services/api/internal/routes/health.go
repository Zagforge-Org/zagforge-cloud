package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func registerHealth(r *router.Router, d *Deps) error {
	g := r.Group()
	return g.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: d.Health.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: d.Health.Readiness},
	})
}
