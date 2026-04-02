package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func inviteSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/orgs/{orgID}/invites", Handler: d.Invite.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/invites", Handler: d.Invite.ListOrgInvites},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/invites/{inviteID}", Handler: d.Invite.Revoke},
		{Method: router.POST, Path: "/auth/invites/accept", Handler: d.Invite.Accept},
	}
}
