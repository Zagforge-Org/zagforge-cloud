package contexturl

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
	"github.com/LegationPro/zagforge/api/internal/service/assembly"
	"github.com/LegationPro/zagforge/shared/go/authclaims"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	"github.com/LegationPro/zagforge/shared/go/storage"
	"github.com/LegationPro/zagforge/shared/go/store"
)

var (
	errNotFound         = errors.New("context token not found")
	errExpired          = errors.New("context token has expired")
	errSnapshotNotFound = errors.New("no snapshot available for this token")
	errSnapshotOutdated = errors.New("snapshot outdated: re-run zigzag --upload to generate a v2 snapshot")
	errAuthRequired     = errors.New("authentication required for this context token")
	errForbidden        = errors.New("you do not have access to this context token")
)

type Handler struct {
	db        *dbpkg.DB
	cache     contextcache.Cache
	github    githubprovider.Worker
	storage   *storage.Client
	log       *zap.Logger
	jwtPubKey ed25519.PublicKey
	jwtIssuer string
}

func NewHandler(db *dbpkg.DB, cache contextcache.Cache, gh githubprovider.Worker, gcs *storage.Client, log *zap.Logger, jwtPubKey ed25519.PublicKey, jwtIssuer string) *Handler {
	return &Handler{db: db, cache: cache, github: gh, storage: gcs, log: log, jwtPubKey: jwtPubKey, jwtIssuer: jwtIssuer}
}

// Head handles HEAD /v1/context/{token} — lightweight token validation, no GitHub fetch.
func (h *Handler) Head(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "token")
	tok, err := h.db.Queries.GetContextTokenByHash(r.Context(), tokenHash(raw))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if tok.ExpiresAt.Valid && tok.ExpiresAt.Time.Before(time.Now()) {
		w.WriteHeader(http.StatusGone)
		return
	}

	if err := h.checkVisibility(r, tok); err != nil {
		if errors.Is(err, errAuthRequired) {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	if !tok.TargetSnapshotID.Valid {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	snap, err := h.db.Queries.GetSnapshotByID(r.Context(), tok.TargetSnapshotID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("X-Snapshot-ID", tok.TargetSnapshotID.String())
	w.Header().Set("X-Commit-SHA", snap.CommitSha)
	w.Header().Set("Content-Type", "text/markdown")
	w.WriteHeader(http.StatusOK)
}

// Get handles GET /v1/context/{token} — assembles and streams report.llm.md.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "token")
	tok, err := h.db.Queries.GetContextTokenByHash(r.Context(), tokenHash(raw))
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errNotFound)
		return
	}
	if tok.ExpiresAt.Valid && tok.ExpiresAt.Time.Before(time.Now()) {
		httputil.ErrResponse(w, http.StatusGone, errExpired)
		return
	}

	if err := h.checkVisibility(r, tok); err != nil {
		if errors.Is(err, errAuthRequired) {
			httputil.ErrResponse(w, http.StatusUnauthorized, errAuthRequired)
		} else {
			httputil.ErrResponse(w, http.StatusForbidden, errForbidden)
		}
		return
	}

	// Update last_used_at async — not on the critical path.
	// Use context.Background() so the goroutine isn't cancelled when the handler returns.
	go func() {
		_ = h.db.Queries.UpdateContextTokenLastUsed(context.Background(), tok.ID)
	}()

	// Resolve snapshot.
	var snap store.Snapshot
	if tok.TargetSnapshotID.Valid {
		snap, err = h.db.Queries.GetSnapshotByID(r.Context(), tok.TargetSnapshotID)
	} else {
		repo, rerr := h.db.Queries.GetRepoByID(r.Context(), tok.RepoID)
		if rerr != nil {
			httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
			return
		}
		snap, err = h.db.Queries.GetLatestSnapshot(r.Context(), store.GetLatestSnapshotParams{
			RepoID: tok.RepoID, Branch: repo.DefaultBranch,
		})
	}
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errSnapshotNotFound)
		return
	}
	if snap.SnapshotVersion < 2 {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errSnapshotOutdated)
		return
	}

	cacheKey := contextcache.Key(tok.RepoID.String(), snap.CommitSha)

	// Cache hit — stream immediately.
	if cached, ok, _ := h.cache.Get(r.Context(), cacheKey); ok {
		w.Header().Set("Content-Type", "text/markdown")

		_, _ = io.WriteString(w, cached)
		return
	}

	// Cache miss — load snapshot metadata from GCS, stream assembly.
	metaBytes, err := h.storage.Download(r.Context(), snap.GcsPath)
	if err != nil {
		h.log.Error("download snapshot from gcs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	var meta struct {
		FileTree []assembly.FileEntry `json:"file_tree"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		h.log.Error("unmarshal snapshot metadata", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	repo, err := h.db.Queries.GetRepoByID(r.Context(), tok.RepoID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, handlerpkg.ErrInternal)
		return
	}

	fetcher := assembly.FetcherFunc(func(ctx context.Context, sha string) (string, error) {
		return h.github.GetBlob(ctx, repo.InstallationID, repo.FullName, sha)
	})

	w.Header().Set("Content-Type", "text/markdown")

	var assembled strings.Builder
	mw := io.MultiWriter(w, &assembled)

	if err := assembly.Assemble(r.Context(), repo.FullName, snap.CommitSha, meta.FileTree, fetcher, mw); err != nil {
		h.log.Error("assembly failed", zap.Error(err))
		return // partial response already flushed
	}

	// Cache the assembled result for the next request.
	go func() {
		_ = h.cache.Set(context.Background(), cacheKey, assembled.String())
	}()
}

// checkVisibility verifies the caller has access to a token based on its visibility mode.
// Returns nil for public tokens. For private/protected, extracts and verifies a JWT
// from the Authorization header, then checks org membership or allowlist.
func (h *Handler) checkVisibility(r *http.Request, tok store.GetContextTokenByHashRow) error {
	if tok.Visibility == store.ContextVisibilityPublic {
		return nil
	}

	// Private and protected tokens require authentication.
	claims, err := h.extractClaims(r)
	if err != nil {
		return errAuthRequired
	}

	userID, err := claims.SubjectUUID()
	if err != nil {
		return errForbidden
	}

	ctx := r.Context()

	switch tok.Visibility {
	case store.ContextVisibilityPrivate:
		// User-scoped token: owner must match.
		if tok.UserID.Valid && tok.UserID == userID {
			return nil
		}
		// Org-scoped token: user must be a member of the org.
		if tok.OrgID.Valid {
			_, err := h.db.Queries.GetMembership(ctx, store.GetMembershipParams{
				UserID: userID,
				OrgID:  tok.OrgID,
			})
			if err == nil {
				return nil
			}
		}
		return errForbidden

	case store.ContextVisibilityProtected:
		allowed, err := h.db.Queries.IsUserAllowedForToken(ctx, store.IsUserAllowedForTokenParams{
			TokenID: tok.ID,
			UserID:  userID,
		})
		if err != nil || !allowed {
			return errForbidden
		}
		return nil
	}

	return errForbidden
}

// extractClaims parses and verifies a JWT from the Authorization header.
// Returns an error if no token is present or the token is invalid.
func (h *Handler) extractClaims(r *http.Request) (*authclaims.Claims, error) {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found || raw == "" {
		return nil, fmt.Errorf("no bearer token")
	}

	claims := &authclaims.Claims{}
	t, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.jwtPubKey, nil
	}, jwt.WithIssuer(h.jwtIssuer))
	if err != nil || !t.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return claims, nil
}
