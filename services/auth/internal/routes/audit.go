package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func auditSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/auth/orgs/{orgID}/audit-logs", Handler: d.Audit.List},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/logins", Handler: d.Audit.LoginMetrics},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/metrics/failed-logins", Handler: d.Audit.FailedLoginMetrics},
	}
}
