package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func repoSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/api/v1/repos", Handler: d.API.ListRepos},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}", Handler: d.API.GetRepo},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs", Handler: d.API.ListJobs},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs/{jobID}", Handler: d.API.GetJob},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots", Handler: d.API.ListSnapshots},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots/latest", Handler: d.API.GetLatestSnapshot},
		{Method: router.GET, Path: "/api/v1/snapshots/{snapshotID}", Handler: d.API.GetSnapshot},
	}
}
