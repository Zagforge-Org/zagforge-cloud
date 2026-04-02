package routes

import "github.com/LegationPro/zagforge/shared/go/router"

func webhookSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/orgs/{orgID}/webhooks", Handler: d.Webhook.Create},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/webhooks", Handler: d.Webhook.List},
		{Method: router.PUT, Path: "/auth/orgs/{orgID}/webhooks/{whID}", Handler: d.Webhook.Update},
		{Method: router.DELETE, Path: "/auth/orgs/{orgID}/webhooks/{whID}", Handler: d.Webhook.Delete},
		{Method: router.GET, Path: "/auth/orgs/{orgID}/webhooks/{whID}/deliveries", Handler: d.Webhook.ListDeliveries},
	}
}
