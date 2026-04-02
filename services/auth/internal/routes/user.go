package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func userSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/auth/me", Handler: d.User.GetMe},
		{Method: router.PUT, Path: "/auth/me", Handler: d.User.UpdateMe},
		{Method: router.PUT, Path: "/auth/me/onboarding", Handler: d.User.UpdateOnboarding},
		{Method: router.GET, Path: "/auth/me/identities", Handler: d.User.ListIdentities},
		{Method: router.DELETE, Path: "/auth/me/identities/{provider}", Handler: d.User.UnlinkIdentity},
	}
}
