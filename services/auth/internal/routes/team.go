package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func teamSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams", Handler: d.Team.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams", Handler: d.Team.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: d.Team.Get},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: d.Team.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}", Handler: d.Team.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: d.Team.ListMembers},
		{Method: router.POST, Path: "/auth/orgs/{orgID}/teams/{teamID}/members", Handler: d.Team.AddMember},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: d.Team.UpdateMemberRole},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/teams/{teamID}/members/{userID}", Handler: d.Team.RemoveMember},
	}
}
