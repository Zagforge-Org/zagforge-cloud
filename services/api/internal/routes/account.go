package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func accountSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/api/v1/account", Handler: d.Account.GetProfile},
		{Method: router.PATCH, Path: "/api/v1/account", Handler: d.Account.UpdateProfile},
		{Method: router.DELETE, Path: "/api/v1/account", Handler: d.Account.DeleteAccount},
		{Method: router.GET, Path: "/api/v1/account/sessions", Handler: d.Account.ListSessions},
		{Method: router.DELETE, Path: "/api/v1/account/sessions/{id}", Handler: d.Account.RevokeSession},
	}
}
