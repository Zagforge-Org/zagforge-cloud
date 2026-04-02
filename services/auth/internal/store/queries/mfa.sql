-- name: UpsertMFASettings :one
INSERT INTO mfa_settings (user_id, totp_secret, totp_enabled)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET totp_secret = $2, totp_enabled = $3
RETURNING *;

-- name: GetMFASettings :one
SELECT * FROM mfa_settings WHERE user_id = $1;

-- name: EnableTOTP :exec
UPDATE mfa_settings
SET totp_enabled = true, totp_verified_at = now()
WHERE user_id = $1;

-- name: DisableTOTP :exec
UPDATE mfa_settings
SET totp_enabled = false, totp_secret = NULL, totp_verified_at = NULL
WHERE user_id = $1;

-- name: CreateBackupCodes :copyfrom
INSERT INTO mfa_backup_codes (user_id, code_hash) VALUES ($1, $2);

-- name: ListUnusedBackupCodes :many
SELECT * FROM mfa_backup_codes
WHERE user_id = $1 AND used_at IS NULL;

-- name: MarkBackupCodeUsed :exec
UPDATE mfa_backup_codes SET used_at = now() WHERE id = $1;

-- name: DeleteBackupCodes :exec
DELETE FROM mfa_backup_codes WHERE user_id = $1;
