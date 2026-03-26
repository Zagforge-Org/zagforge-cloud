package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	"github.com/LegationPro/zagforge/auth/internal/service/audit"
	oauthsvc "github.com/LegationPro/zagforge/auth/internal/service/oauth"
	sessionsvc "github.com/LegationPro/zagforge/auth/internal/service/session"
	"github.com/LegationPro/zagforge/auth/internal/service/token"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// Callback handles the OAuth callback, creates or links the user, and issues tokens.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		httputil.ErrResponse(w, http.StatusBadRequest, errUnsupportedProvider)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		httputil.ErrResponse(w, http.StatusBadRequest, errMissingCodeOrState)
		return
	}

	// Validate and consume state.
	oauthState, err := h.db.Queries.GetAndDeleteOAuthState(r.Context(), state)
	if err != nil {
		h.log.Warn("invalid oauth state", zap.String("state", state), zap.Error(err))
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidState)
		return
	}

	if oauthState.Provider != provider {
		httputil.ErrResponse(w, http.StatusBadRequest, errStateMismatch)
		return
	}

	// Exchange code for tokens and user info.
	accessToken, refreshOAuthToken, userInfo, err := p.Exchange(r.Context(), code)
	if err != nil {
		h.log.Error("oauth exchange", zap.Error(err))
		httputil.ErrResponse(w, http.StatusBadGateway, errOAuthExchangeFailed)
		return
	}

	// Resolve or create user.
	user, err := h.resolveUser(r.Context(), provider, userInfo)
	if err != nil {
		h.log.Error("resolve user", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	// Store/update encrypted OAuth tokens.
	if err := h.storeOAuthTokens(r.Context(), provider, userInfo, accessToken, refreshOAuthToken); err != nil {
		h.log.Error("store oauth tokens", zap.Error(err))
	}

	// Check MFA — if enabled, return challenge token instead.
	mfa, err := h.db.Queries.GetMFASettings(r.Context(), user.ID)
	if err == nil && mfa.TotpEnabled {
		challengeToken, err := h.tokenSvc.IssueMFAChallengeToken(httputil.UUIDToString(user.ID))
		if err != nil {
			h.log.Error("issue mfa challenge", zap.Error(err))
			httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
			return
		}
		redirectURL := oauthState.RedirectUri + "?mfa_required=true&mfa_token=" + challengeToken
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// Create session and issue tokens.
	if err := h.issueTokensAndRedirect(w, r, user, oauthState.RedirectUri); err != nil {
		h.log.Error("issue tokens", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	h.auditSvc.Log(r.Context(), audit.LogParams{
		ActorID:  user.ID,
		Action:   audit.ActionUserLogin,
		Request:  r,
		Metadata: fmt.Appendf(nil, `{"provider":"%s"}`, provider),
	})
}

func (h *Handler) resolveUser(ctx context.Context, provider string, info oauthsvc.UserInfo) (authstore.User, error) {
	// Check if OAuth identity already exists.
	identity, err := h.db.Queries.GetOAuthIdentity(ctx, authstore.GetOAuthIdentityParams{
		Provider:   provider,
		ProviderID: info.ProviderID,
	})
	if err == nil {
		user, err := h.db.Queries.GetUserByID(ctx, identity.UserID)
		if err != nil {
			return authstore.User{}, fmt.Errorf("get user by id: %w", err)
		}
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return authstore.User{}, fmt.Errorf("get oauth identity: %w", err)
	}

	// Check if user exists by email.
	if info.Email != "" {
		user, err := h.db.Queries.GetUserByEmail(ctx, info.Email)
		if err == nil {
			// Link this provider to existing user.
			_, err = h.db.Queries.CreateOAuthIdentity(ctx, authstore.CreateOAuthIdentityParams{
				UserID:      user.ID,
				Provider:    provider,
				ProviderID:  info.ProviderID,
				Email:       pgtype.Text{String: info.Email, Valid: true},
				DisplayName: pgtype.Text{String: info.DisplayName, Valid: true},
				AvatarUrl:   pgtype.Text{String: info.AvatarURL, Valid: true},
			})
			if err != nil {
				return authstore.User{}, fmt.Errorf("link oauth identity: %w", err)
			}
			return user, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return authstore.User{}, fmt.Errorf("get user by email: %w", err)
		}
	}

	// Create new user.
	nameParts := strings.SplitN(info.DisplayName, " ", 2)
	firstName := nameParts[0]
	var lastName string
	if len(nameParts) > 1 {
		lastName = nameParts[1]
	}

	user, err := h.db.Queries.CreateUser(ctx, authstore.CreateUserParams{
		Email:         info.Email,
		EmailVerified: info.Email != "",
		FirstName:     pgtype.Text{String: firstName, Valid: firstName != ""},
		LastName:      pgtype.Text{String: lastName, Valid: lastName != ""},
		AvatarUrl:     pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		return authstore.User{}, fmt.Errorf("create user: %w", err)
	}

	// Create OAuth identity for the new user.
	_, err = h.db.Queries.CreateOAuthIdentity(ctx, authstore.CreateOAuthIdentityParams{
		UserID:      user.ID,
		Provider:    provider,
		ProviderID:  info.ProviderID,
		Email:       pgtype.Text{String: info.Email, Valid: info.Email != ""},
		DisplayName: pgtype.Text{String: info.DisplayName, Valid: info.DisplayName != ""},
		AvatarUrl:   pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		return authstore.User{}, fmt.Errorf("create oauth identity: %w", err)
	}

	return user, nil
}

func (h *Handler) storeOAuthTokens(ctx context.Context, provider string, info oauthsvc.UserInfo, accessToken, refreshToken string) error {
	var encAccess, encRefresh []byte
	var err error

	if accessToken != "" {
		encAccess, err = h.encSvc.Encrypt([]byte(accessToken))
		if err != nil {
			return err
		}
	}
	if refreshToken != "" {
		encRefresh, err = h.encSvc.Encrypt([]byte(refreshToken))
		if err != nil {
			return err
		}
	}

	return h.db.Queries.UpdateOAuthTokens(ctx, authstore.UpdateOAuthTokensParams{
		Provider:     provider,
		ProviderID:   info.ProviderID,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
	})
}

func (h *Handler) issueTokensAndRedirect(w http.ResponseWriter, r *http.Request, user authstore.User, redirectURI string) error {
	session, err := h.sessionSvc.Create(r.Context(), sessionsvc.CreateParams{
		UserID:  user.ID,
		Request: r,
	})
	if err != nil {
		return err
	}

	// Generate refresh token.
	refreshTok, err := h.tokenSvc.GenerateRefreshToken()
	if err != nil {
		return err
	}

	_, err = h.db.Queries.CreateRefreshToken(r.Context(), authstore.CreateRefreshTokenParams{
		UserID:    user.ID,
		SessionID: session.ID,
		TokenHash: refreshTok.Hash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(h.tokenSvc.RefreshTokenTTL()), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	// Issue access token.
	name := ""
	if user.FirstName.Valid {
		name = user.FirstName.String
	}
	if user.LastName.Valid {
		name += " " + user.LastName.String
	}

	accessJWT, err := h.tokenSvc.IssueAccessToken(token.AccessTokenParams{
		UserID: httputil.UUIDToString(user.ID),
		Email:  user.Email,
		Name:   strings.TrimSpace(name),
		MFA:    false,
	})
	if err != nil {
		return fmt.Errorf("issue access token: %w", err)
	}

	// Set refresh token as HttpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTok.Raw,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.tokenSvc.RefreshTokenTTL().Seconds()),
	})

	redirectURL := redirectURI + "?access_token=" + accessJWT
	http.Redirect(w, r, redirectURL, http.StatusFound)
	return nil
}
