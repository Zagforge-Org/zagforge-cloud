package mfa

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	mfasvc "github.com/LegationPro/zagforge/auth/internal/service/mfa"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

type challengeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Challenge validates a TOTP code during the login flow using the MFA challenge token.
// On success, issues a full access + refresh token pair.
func (h *Handler) Challenge(w http.ResponseWriter, r *http.Request) {
	body, err := httputil.DecodeJSON[challengeRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	// Validate the MFA challenge token.
	userIDStr, err := h.tokenSvc.ValidateMFAChallengeToken(body.MFAToken)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, errInvalidToken)
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(userIDStr); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidToken)
		return
	}

	// Get MFA settings and decrypt secret.
	settings, err := h.db.Queries.GetMFASettings(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.ErrResponse(w, http.StatusBadRequest, errMFANotEnabled)
			return
		}
		h.log.Error("get mfa settings", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}
	if !settings.TotpEnabled {
		httputil.ErrResponse(w, http.StatusBadRequest, errMFANotEnabled)
		return
	}

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

	// MFA verified — issue tokens.
	if err := h.issueTokens(w, r, userID); err != nil {
		h.log.Error("issue tokens after mfa", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID:  userID,
		Action:   audit.ActionUserLogin,
		Request:  r,
		Metadata: []byte(`{"mfa":"totp"}`),
	})
}

// BackupCodeVerify validates a backup code during the login flow.
func (h *Handler) BackupCodeVerify(w http.ResponseWriter, r *http.Request) {
	body, err := httputil.DecodeJSON[backupVerifyRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if err := validate.Struct(body); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	userIDStr, err := h.tokenSvc.ValidateMFAChallengeToken(body.MFAToken)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, errInvalidToken)
		return
	}

	var userID pgtype.UUID
	if err := userID.Scan(userIDStr); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidToken)
		return
	}

	// Find matching backup code.
	codes, err := h.db.Queries.ListUnusedBackupCodes(r.Context(), userID)
	if err != nil {
		h.log.Error("list backup codes", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	matched := false
	for _, code := range codes {
		if mfasvc.VerifyBackupCode(body.Code, code.CodeHash) == nil {
			_ = h.db.Queries.MarkBackupCodeUsed(r.Context(), code.ID)
			matched = true
			break
		}
	}
	if !matched {
		httputil.ErrResponse(w, http.StatusUnauthorized, errInvalidBackup)
		return
	}

	if err := h.issueTokens(w, r, userID); err != nil {
		h.log.Error("issue tokens after backup", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID:  userID,
		Action:   audit.ActionUserLogin,
		Request:  r,
		Metadata: []byte(`{"mfa":"backup_code"}`),
	})
}

func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, userID pgtype.UUID) error {
	user, err := h.db.Queries.GetUserByID(r.Context(), userID)
	if err != nil {
		return err
	}

	session, err := h.sessionSvc.Create(r.Context(), sessionsvc.CreateParams{
		UserID:  userID,
		Request: r,
	})
	if err != nil {
		return err
	}

	refreshTok, err := h.tokenSvc.GenerateRefreshToken()
	if err != nil {
		return err
	}

	_, err = h.db.Queries.CreateRefreshToken(r.Context(), authstore.CreateRefreshTokenParams{
		UserID:    userID,
		SessionID: session.ID,
		TokenHash: refreshTok.Hash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(h.tokenSvc.RefreshTokenTTL()), Valid: true},
	})
	if err != nil {
		return err
	}

	name := ""
	if user.FirstName.Valid {
		name = user.FirstName.String
	}
	if user.LastName.Valid {
		name += " " + user.LastName.String
	}

	accessJWT, err := h.tokenSvc.IssueAccessToken(token.AccessTokenParams{
		UserID: httputil.UUIDToString(userID),
		Email:  user.Email,
		Name:   strings.TrimSpace(name),
		MFA:    true,
	})
	if err != nil {
		return err
	}

	// Set refresh cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTok.Raw,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.tokenSvc.RefreshTokenTTL().Seconds()),
	})

	httputil.OkResponse(w, challengeResponse{
		AccessToken: accessJWT,
		ExpiresIn:   900,
	})
	return nil
}
