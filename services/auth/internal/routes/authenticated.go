package routes

import (
	"time"

	authmw "github.com/LegationPro/zagforge/auth/internal/middleware/auth"
	"github.com/LegationPro/zagforge/auth/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerAuthenticated(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(authmw.Auth(d.PubKey, d.JWTIssuer, d.Log))
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.Config{
		MaxRequests: 60,
		Window:      1 * time.Minute,
	}, "auth", d.Log))

	var all []router.Subroute
	all = append(all, sessionAuthSubroutes(d)...)
	all = append(all, mfaAuthSubroutes(d)...)
	all = append(all, userSubroutes(d)...)
	all = append(all, orgSubroutes(d)...)
	all = append(all, inviteSubroutes(d)...)
	all = append(all, teamSubroutes(d)...)
	all = append(all, auditSubroutes(d)...)
	all = append(all, webhookSubroutes(d)...)
	all = append(all, adminSubroutes(d)...)

	return g.Create(all)
}

func sessionAuthSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/logout/all", Handler: d.Session.LogoutAll},
		{Method: router.GET, Path: "/auth/sessions", Handler: d.Session.ListSessions},
		{Method: router.DELETE, Path: "/auth/sessions/{sessionID}", Handler: d.Session.RevokeSession},
	}
}
