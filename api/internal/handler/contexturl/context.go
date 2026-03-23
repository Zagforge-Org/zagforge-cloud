package contexturl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/service/assembly"
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
	errInternal         = errors.New("internal error")
)

type Handler struct {
	db      *dbpkg.DB
	cache   contextcache.Cache
	github  githubprovider.Worker
	storage *storage.Client
	log     *zap.Logger
}

func NewHandler(db *dbpkg.DB, cache contextcache.Cache, gh githubprovider.Worker, gcs *storage.Client, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, github: gh, storage: gcs, log: log}
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
			httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
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
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	var meta struct {
		FileTree []assembly.FileEntry `json:"file_tree"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		h.log.Error("unmarshal snapshot metadata", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	repo, err := h.db.Queries.GetRepoByID(r.Context(), tok.RepoID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	// GitHub blob fetcher.
	// NOTE: requires GetBlob(ctx, installationID, repoFullName, sha) on the GitHub provider.
	// If this method doesn't exist on githubprovider.Worker, add it in shared/go/provider/github/api.go.
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
