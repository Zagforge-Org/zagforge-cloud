package routes

import (
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerHealth(r *router.Router, d *Deps) error {
	// Health — no auth, no rate limit.
	healthRoutes := r.Group()
	if err := healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: d.Health.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: d.Health.Readiness},
	}); err != nil {
		return err
	}

	// JWKS — public, no auth.
	jwks := r.Group()
	if err := jwks.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/.well-known/jwks.json", Handler: d.OAuth.JWKS},
	}); err != nil {
		return err
	}

	// Public invite lookup — no auth (token is the secret).
	invitePublic := r.Group()
	return invitePublic.Create([]router.Subroute{
		{Method: router.GET, Path: "/auth/invites/{token}", Handler: d.Invite.GetByToken},
	})
}
