package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func settingsSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: d.CtxTokens.List},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/context-tokens", Handler: d.CtxTokens.Create},
		{Method: router.DELETE, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}", Handler: d.CtxTokens.Delete},
		{Method: router.PUT, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}/allowed-users", Handler: d.CtxTokens.UpdateAllowedUsers},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/context-tokens/{tokenID}/allowed-users", Handler: d.CtxTokens.ListAllowedUsers},
		{Method: router.GET, Path: "/api/v1/orgs/settings/ai-keys", Handler: d.AIKeys.List},
		{Method: router.PUT, Path: "/api/v1/orgs/settings/ai-keys", Handler: d.AIKeys.Upsert},
		{Method: router.DELETE, Path: "/api/v1/orgs/settings/ai-keys/{provider}", Handler: d.AIKeys.Delete},
		{Method: router.GET, Path: "/api/v1/orgs/settings/cli-keys", Handler: d.CLIKeys.List},
		{Method: router.POST, Path: "/api/v1/orgs/settings/cli-keys", Handler: d.CLIKeys.Create},
		{Method: router.DELETE, Path: "/api/v1/orgs/settings/cli-keys/{keyID}", Handler: d.CLIKeys.Delete},
		{Method: router.POST, Path: "/api/v1/repos/{repoID}/query", Handler: d.Query.Query},
	}
}
