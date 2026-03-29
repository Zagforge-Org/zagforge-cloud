package routes

import (
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerAPIv1(r *router.Router, d *Deps) error {
	v1 := r.Group()
	v1.Use(auth.Auth(d.JWTPubKey, d.JWTIssuer, d.Log))
	v1.Use(auth.Scope(d.Queries, d.Log))
	v1.Use(ratelimit.RateLimit(d.RDB, ratelimit.RateLimitConfig{
		MaxRequests: rateLimitAPI,
		Window:      rateLimitWindow,
	}, "api", d.Log))

	var all []router.Subroute
	all = append(all, repoSubroutes(d)...)
	all = append(all, settingsSubroutes(d)...)
	all = append(all, accountSubroutes(d)...)
	all = append(all, orgSubroutes(d)...)

	return v1.Create(all)
}
