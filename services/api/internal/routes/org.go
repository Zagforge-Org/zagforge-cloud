package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func orgSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/api/v1/orgs", Handler: d.Org.CreateOrg},
		{Method: router.GET, Path: "/api/v1/orgs", Handler: d.Org.ListOrgs},
		{Method: router.PATCH, Path: "/api/v1/orgs/{orgID}", Handler: d.Org.UpdateOrg},
		{Method: router.DELETE, Path: "/api/v1/orgs/{orgID}", Handler: d.Org.DeleteOrg},
		{Method: router.GET, Path: "/api/v1/orgs/{orgID}/members", Handler: d.Org.ListMembers},
		{Method: router.POST, Path: "/api/v1/orgs/{orgID}/members", Handler: d.Org.InviteMember},
		{Method: router.PATCH, Path: "/api/v1/orgs/{orgID}/members/{userID}", Handler: d.Org.UpdateMemberRole},
		{Method: router.DELETE, Path: "/api/v1/orgs/{orgID}/members/{userID}", Handler: d.Org.RemoveMember},
		{Method: router.GET, Path: "/api/v1/orgs/{orgID}/audit-log", Handler: d.Org.ListAuditLog},
	}
}
