package role

// Org-level roles.
const (
	OrgOwner  = "owner"
	OrgAdmin  = "admin"
	OrgMember = "member"
)

// OrgAdminOrAbove are roles that can manage org resources.
var OrgAdminOrAbove = []string{OrgOwner, OrgAdmin}

// Team-level roles.
const (
	TeamLead   = "lead"
	TeamMember = "member"
)
