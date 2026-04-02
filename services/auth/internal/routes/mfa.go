package routes

import (
	"time"

	"github.com/LegationPro/zagforge/auth/internal/middleware/ratelimit"
	"github.com/LegationPro/zagforge/shared/go/router"
)

func registerMFAPublic(r *router.Router, d *Deps) error {
	g := r.Group()
	g.Use(ratelimit.RateLimit(d.RDB, ratelimit.Config{
		MaxRequests: 10,
		Window:      1 * time.Minute,
	}, "mfa", d.Log))
	return g.Create([]router.Subroute{
		{Method: router.POST, Path: "/auth/mfa/totp/challenge", Handler: d.MFA.Challenge},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/verify", Handler: d.MFA.BackupCodeVerify},
	})
}

// mfaAuthSubroutes returns authenticated MFA subroutes (setup, verify, disable).
func mfaAuthSubroutes(d *Deps) []router.Subroute {
	return []router.Subroute{
		{Method: router.POST, Path: "/auth/mfa/totp/setup", Handler: d.MFA.Setup},
		{Method: router.POST, Path: "/auth/mfa/totp/verify", Handler: d.MFA.Verify},
		{Method: router.POST, Path: "/auth/mfa/totp/disable", Handler: d.MFA.Disable},
		{Method: router.POST, Path: "/auth/mfa/backup-codes/generate", Handler: d.MFA.RegenerateBackupCodes},
	}
}
