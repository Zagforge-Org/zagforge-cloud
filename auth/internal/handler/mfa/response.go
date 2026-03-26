package mfa

type setupResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type verifyResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

type backupCodesResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

type verifyRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

type challengeRequest struct {
	MFAToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code" validate:"required,len=6"`
}

type backupVerifyRequest struct {
	MFAToken string `json:"mfa_token" validate:"required"`
	Code     string `json:"code" validate:"required"`
}
