package mfa

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	mfasvc "github.com/LegationPro/zagforge/auth/internal/service/mfa"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Setup generates a new TOTP secret for the user. Does not enable MFA yet.
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	userID, email, err := userFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	// Check if already enabled.
	existing, err := h.db.Queries.GetMFASettings(r.Context(), userID)
	if err == nil && existing.TotpEnabled {
		httputil.ErrResponse(w, http.StatusConflict, errMFAAlreadyActive)
		return
	}

	key, err := mfasvc.GenerateTOTPKey(email)
	if err != nil {
		h.log.Error("generate totp key", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	// Encrypt and store the secret (not yet enabled).
	encSecret, err := h.encSvc.Encrypt([]byte(key.Secret))
	if err != nil {
		h.log.Error("encrypt totp secret", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	_, err = h.db.Queries.UpsertMFASettings(r.Context(), authstore.UpsertMFASettingsParams{
		UserID:      userID,
		TotpSecret:  encSecret,
		TotpEnabled: false,
	})
	if err != nil {
		h.log.Error("upsert mfa settings", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, setupResponse{
		Secret: key.Secret,
		URI:    key.URI,
	})
}

// Verify validates a TOTP code and enables MFA. Returns backup codes.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	userID, _, err := userFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	body, err := httputil.DecodeJSON[verifyRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	// Get stored (encrypted) secret.
	settings, err := h.db.Queries.GetMFASettings(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.ErrResponse(w, http.StatusBadRequest, errMFANotSetup)
			return
		}
		h.log.Error("get mfa settings", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	if settings.TotpEnabled {
		httputil.ErrResponse(w, http.StatusConflict, errMFAAlreadyActive)
		return
	}

	// Decrypt and validate.
	secret, err := h.encSvc.Decrypt(settings.TotpSecret)
	if err != nil {
		h.log.Error("decrypt totp secret", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	if err := mfasvc.ValidateTOTPCode(body.Code, string(secret)); err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, errInvalidCode)
		return
	}

	// Enable MFA.
	if err := h.db.Queries.EnableTOTP(r.Context(), userID); err != nil {
		h.log.Error("enable totp", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	// Generate backup codes.
	codes, err := generateAndStoreBackupCodes(r, h.db.Queries, userID)
	if err != nil {
		h.log.Error("generate backup codes", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID: userID,
		Action:  audit.ActionMFAEnabled,
		Request: r,
	})

	httputil.OkResponse(w, verifyResponse{BackupCodes: codes})
}

// Disable turns off MFA for the user.
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	userID, _, err := userFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	settings, err := h.db.Queries.GetMFASettings(r.Context(), userID)
	if err != nil || !settings.TotpEnabled {
		httputil.ErrResponse(w, http.StatusBadRequest, errMFANotEnabled)
		return
	}

	if err := h.db.Queries.DisableTOTP(r.Context(), userID); err != nil {
		h.log.Error("disable totp", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	_ = h.db.Queries.DeleteBackupCodes(r.Context(), userID)

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID: userID,
		Action:  audit.ActionMFADisabled,
		Request: r,
	})

	httputil.WriteJSON(w, http.StatusOK, handler.StatusResponse{Status: "disabled"})
}

func userFromContext(r *http.Request) (pgtype.UUID, string, error) {
	claims, err := authclaims.FromContext(r.Context())
	if err != nil {
		return pgtype.UUID{}, "", err
	}
	id, err := claims.SubjectUUID()
	if err != nil {
		return pgtype.UUID{}, "", err
	}
	return id, claims.Email, nil
}
