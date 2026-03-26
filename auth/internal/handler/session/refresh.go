package session

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/auth/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Refresh exchanges a refresh token for a new access token + refresh token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken := extractRefreshToken(r)
	if rawToken == "" {
		httputil.ErrResponse(w, http.StatusBadRequest, errMissingRefreshToken)
		return
	}

	if err := validate.Struct(refreshRequest{RefreshToken: rawToken}); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	tokenHash := token.HashToken(rawToken)

	rt, err := h.db.Queries.GetRefreshTokenByHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.ErrResponse(w, http.StatusUnauthorized, errInvalidRefreshToken)
			return
		}
		h.log.Error("get refresh token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	if rt.RevokedAt.Valid || (rt.ExpiresAt.Valid && rt.ExpiresAt.Time.Before(time.Now())) {
		httputil.ErrResponse(w, http.StatusUnauthorized, errRefreshTokenExpired)
		return
	}

	sess, err := h.db.Queries.GetSessionByID(r.Context(), rt.SessionID)
	if err != nil || sess.RevokedAt.Valid || (sess.ExpiresAt.Valid && sess.ExpiresAt.Time.Before(time.Now())) {
		httputil.ErrResponse(w, http.StatusUnauthorized, errSessionExpired)
		return
	}

	// Rotate: revoke old, issue new.
	if err := h.db.Queries.RevokeRefreshToken(r.Context(), rt.ID); err != nil {
		h.log.Error("revoke old refresh token", zap.Error(err))
	}

	refreshTok, err := h.tokenSvc.GenerateRefreshToken()
	if err != nil {
		h.log.Error("generate refresh token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	_, err = h.db.Queries.CreateRefreshToken(r.Context(), authstore.CreateRefreshTokenParams{
		UserID:    rt.UserID,
		SessionID: rt.SessionID,
		TokenHash: refreshTok.Hash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(h.tokenSvc.RefreshTokenTTL()), Valid: true},
	})
	if err != nil {
		h.log.Error("store new refresh token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	_ = h.db.Queries.UpdateSessionLastActive(r.Context(), rt.SessionID)

	user, err := h.db.Queries.GetUserByID(r.Context(), rt.UserID)
	if err != nil {
		h.log.Error("get user for refresh", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	orgClaim := resolveOrgClaim(r, h.db.Queries, user.ID)
	name := buildFullName(user)

	mfaEnabled := false
	mfa, err := h.db.Queries.GetMFASettings(r.Context(), user.ID)
	if err == nil {
		mfaEnabled = mfa.TotpEnabled
	}

	accessJWT, err := h.tokenSvc.IssueAccessToken(token.AccessTokenParams{
		UserID: httputil.UUIDToString(user.ID),
		Email:  user.Email,
		Name:   name,
		Org:    orgClaim,
		MFA:    mfaEnabled,
	})
	if err != nil {
		h.log.Error("issue access token", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	setRefreshCookie(w, r, refreshTok.Raw, h.tokenSvc.RefreshTokenTTL())

	httputil.OkResponse(w, tokenResponse{
		AccessToken: accessJWT,
		ExpiresIn:   900,
	})
}
