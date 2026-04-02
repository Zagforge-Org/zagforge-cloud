package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func adminSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/auth/admin/users", Handler: d.Admin.ListUsers},
		{Method: router.GET, Path: "/auth/admin/users/{userID}", Handler: d.Admin.GetUser},
		{Method: router.PUT, Path: "/auth/admin/users/{userID}", Handler: d.Admin.UpdateUser},
		{Method: router.GET, Path: "/auth/admin/orgs", Handler: d.Admin.ListOrgs},
		{Method: router.PUT, Path: "/auth/admin/orgs/{orgID}", Handler: d.Admin.UpdateOrgPlan},
	}
}
