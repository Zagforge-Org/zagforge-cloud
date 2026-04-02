package routes

import "time"

const (
	bodyLimit1MB  = 1 << 20
	bodyLimit10MB = 10 << 20

	rateLimitOAuth   = 30
	rateLimitWebhook = 120
	rateLimitAPI     = 60
	rateLimitUpload  = 60

	rateLimitWindow = 1 * time.Minute
)
