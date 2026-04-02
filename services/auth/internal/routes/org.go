package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func orgSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/orgs", Handler: d.Org.Create},
		{Method: router.GET, Path: "/auth/orgs", Handler: d.Org.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}", Handler: d.Org.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}", Handler: d.Org.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}", Handler: d.Org.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/members", Handler: d.Org.ListMembers},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: d.Org.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/members/{userID}", Handler: d.Org.RemoveMember},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/transfer", Handler: d.Org.TransferOwnership},
	}
}
