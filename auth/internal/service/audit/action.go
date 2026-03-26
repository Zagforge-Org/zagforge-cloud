package audit

// Action constants for audit log events.
const (
	ActionUserLogin     = "user.login"
	ActionUserLogout    = "user.logout"
	ActionUserLogoutAll = "user.logout_all"

	ActionMFAEnabled  = "mfa.enabled"
	ActionMFADisabled = "mfa.disabled"

	ActionOrgCreated           = "org.created"
	ActionOrgUpdated           = "org.updated"
	ActionOrgDeleted           = "org.deleted"
	ActionOrgMemberAdded       = "org.member.added"
	ActionOrgMemberRemoved     = "org.member.removed"
	ActionOrgMemberRoleChanged = "org.member.role_changed"
	ActionOrgOwnerTransferred  = "org.owner.transferred"

	ActionTeamCreated       = "team.created"
	ActionTeamUpdated       = "team.updated"
	ActionTeamDeleted       = "team.deleted"
	ActionTeamMemberAdded   = "team.member.added"
	ActionTeamMemberRemoved = "team.member.removed"

	ActionInviteCreated  = "invite.created"
	ActionInviteAccepted = "invite.accepted"
	ActionInviteRevoked  = "invite.revoked"

	ActionSessionRevoked = "session.revoked"
)

// Target type constants for audit log entries.
const (
	TargetTypeUser    = "user"
	TargetTypeOrg     = "org"
	TargetTypeTeam    = "team"
	TargetTypeSession = "session"
	TargetTypeInvite  = "invite"
)
