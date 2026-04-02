package routes

import (
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerContext(r *router.Router, d *Deps) error {
	g := r.Group()
	return g.Create([]router.Subroute{
		{Method: router.HEAD, Path: "/v1/context/{token}", Handler: d.ContextURL.Head},
		{Method: router.GET, Path: "/v1/context/{token}", Handler: d.ContextURL.Get},
	})
}
