package query

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/middleware/auth"
	"github.com/LegationPro/zagforge/api/internal/service/aiprovider"
	"github.com/LegationPro/zagforge/api/internal/service/assembly"
	"github.com/LegationPro/zagforge/api/internal/service/encryption"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	githubprovider "github.com/LegationPro/zagforge/shared/go/provider/github"
	"github.com/LegationPro/zagforge/shared/go/storage"
	store "github.com/LegationPro/zagforge/shared/go/store"
)

// Handler handles POST /api/v1/{org}/{repo}/query.
type Handler struct {
	db      *dbpkg.DB
	cache   contextcache.Cache
	github  githubprovider.Worker
	storage *storage.Client
	enc     *encryption.Service
	log     *zap.Logger
}

func NewHandler(db *dbpkg.DB, cache contextcache.Cache, gh githubprovider.Worker, gcs *storage.Client, enc *encryption.Service, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, github: gh, storage: gcs, enc: enc, log: log}
}

// Query handles the query console — assembles context, streams AI response via SSE.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	req, err := httputil.DecodeJSON[queryRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, errInvalidBody)
		return
	}
	if err := validate.Struct(req); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	orgID := auth.OrgIDFromContext(r.Context())

	provider, rawKey, err := h.selectProvider(r.Context(), orgID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errNoAIKey)
		return
	}

	repoID, err := httputil.ParseUUID(r, "repoID")
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}
	repo, err := h.db.Queries.GetRepoByID(r.Context(), repoID)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotFound)
		return
	}
	if repo.OrgID != orgID {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotFound)
		return
	}

	snap, err := h.resolveSnapshot(r.Context(), req.SnapshotID, repoID, repo.DefaultBranch)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errSnapshotNotFound)
		return
	}
	if snap.SnapshotVersion < 2 {
		httputil.ErrResponse(w, http.StatusUnprocessableEntity, errSnapshotOutdated)
		return
	}

	contextMD, err := h.getOrAssembleContext(r.Context(), repo, snap)
	if err != nil {
		h.log.Error("assemble context", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 5-minute timeout for AI streaming — prevents hung connections.
	streamCtx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf("%s\n\n---\n\n%s\n\n---\n\nUser: %s", SystemPrompt, contextMD, req.Question)
	if err := h.streamAI(streamCtx, w, provider, string(rawKey), prompt); err != nil {
		h.log.Error("stream ai", zap.Error(err), zap.String("provider", provider))
	}
}

// selectProvider finds the first configured AI provider key for the org.
func (h *Handler) selectProvider(ctx context.Context, orgID pgtype.UUID) (string, []byte, error) {
	for _, p := range ProviderOrder {
		k, err := h.db.Queries.GetAIProviderKeyForOrg(ctx, store.GetAIProviderKeyForOrgParams{
			OrgID: orgID, Provider: p,
		})
		if err != nil {
			continue
		}
		decrypted, err := h.enc.Decrypt(k.KeyCipher)
		if err != nil {
			h.log.Warn("decrypt ai key failed", zap.String("provider", p), zap.Error(err))
			continue
		}
		return p, decrypted, nil
	}
	return "", nil, errNoAIKey
}

// resolveSnapshot resolves a snapshot by explicit ID or latest for default branch.
func (h *Handler) resolveSnapshot(ctx context.Context, snapshotID *string, repoID pgtype.UUID, defaultBranch string) (store.Snapshot, error) {
	if snapshotID != nil {
		snapID, err := httputil.UUIDFromString(*snapshotID)
		if err != nil {
			return store.Snapshot{}, err
		}
		return h.db.Queries.GetSnapshotByID(ctx, snapID)
	}
	return h.db.Queries.GetLatestSnapshot(ctx, store.GetLatestSnapshotParams{
		RepoID: repoID, Branch: defaultBranch,
	})
}

// getOrAssembleContext returns cached context or assembles from GCS + GitHub.
func (h *Handler) getOrAssembleContext(ctx context.Context, repo store.Repository, snap store.Snapshot) (string, error) {
	cacheKey := contextcache.Key(repo.ID.String(), snap.CommitSha)

	if cached, ok, _ := h.cache.Get(ctx, cacheKey); ok {
		return cached, nil
	}

	metaBytes, err := h.storage.Download(ctx, snap.GcsPath)
	if err != nil {
		return "", fmt.Errorf("download snapshot: %w", err)
	}

	var meta struct {
		FileTree []assembly.FileEntry `json:"file_tree"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return "", fmt.Errorf("unmarshal snapshot: %w", err)
	}

	fetcher := assembly.FetcherFunc(func(ctx context.Context, sha string) (string, error) {
		return h.github.GetBlob(ctx, repo.InstallationID, repo.FullName, sha)
	})

	var assembled strings.Builder
	if err := assembly.Assemble(ctx, repo.FullName, snap.CommitSha, meta.FileTree, fetcher, &assembled); err != nil {
		return "", fmt.Errorf("assemble: %w", err)
	}

	result := assembled.String()
	go func() {
		_ = h.cache.Set(context.Background(), cacheKey, result)
	}()

	return result, nil
}

// streamAI selects the AI provider and streams the response via SSE.
func (h *Handler) streamAI(ctx context.Context, w http.ResponseWriter, providerName, apiKey, prompt string) error {
	p, err := aiprovider.New(providerName)
	if err != nil {
		return err
	}
	return p.Stream(ctx, w, apiKey, prompt)
}
