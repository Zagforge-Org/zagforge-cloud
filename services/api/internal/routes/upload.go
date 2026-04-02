package routes

import (
	"github.com/LegationPro/zagforge/api/internal/middleware/bodylimit"
	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/contenttype"
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerUpload(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(bodylimit.Limit(bodyLimit10MB))
	g.Use(contenttype.RequireJSON())
	g.Use(clitoken.Auth(d.Queries, d.CLIAPIKey, d.Log))
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitUpload,
		Window:      rateLimitWindow,
	}, "upload", d.Log))
	return g.Create([]router.Subroute{
		{Method: router.POST, Path: "/api/v1/upload", Handler: d.Upload.Upload},
	})
}
