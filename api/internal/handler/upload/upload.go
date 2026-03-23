package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	"github.com/LegationPro/zagforge/api/internal/validate"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/storage"
	"github.com/LegationPro/zagforge/shared/go/store"
	"go.uber.org/zap"
)

var (
	errSnapshotVersion  = errors.New("metadata_snapshot.snapshot_version must be 2")
	errOrgNotFound      = errors.New("organization not found")
	errRepoNotConnected = errors.New("repository not connected; install the Zagforge GitHub App first")
	errInternal         = errors.New("internal error")
)

type snapshotMetadata struct {
	SnapshotVersion int    `json:"snapshot_version" validate:"required,gte=2"`
	ZigzagVersion   string `json:"zigzag_version" validate:"required"`
	CommitSha       string `json:"commit_sha" validate:"required"`
	Branch          string `json:"branch" validate:"required"`
	Summary         any    `json:"summary"`
	FileTree        []struct {
		Path     string `json:"path" validate:"required"`
		Language string `json:"language"`
		Lines    int    `json:"lines"`
		SHA      string `json:"sha" validate:"required"`
	} `json:"file_tree" validate:"required,min=1,max=10000,dive"`
}

type uploadResponse struct {
	SnapshotID string    `json:"snapshot_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type uploadRequest struct {
	OrgSlug          string           `json:"org_slug" validate:"required"`
	RepoFullName     string           `json:"repo_full_name" validate:"required"`
	CommitSha        string           `json:"commit_sha" validate:"required,min=7,max=40"`
	Branch           string           `json:"branch" validate:"required"`
	MetadataSnapshot snapshotMetadata `json:"metadata_snapshot" validate:"required"`
}

// Handler handles CLI snapshot uploads.
type Handler struct {
	db      *dbpkg.DB
	storage *storage.Client
	log     *zap.Logger
}

func NewHandler(db *dbpkg.DB, gcs *storage.Client, log *zap.Logger) *Handler {
	return &Handler{db: db, storage: gcs, log: log}
}

// Upload handles POST /api/v1/upload.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	req, err := httputil.DecodeJSON[uploadRequest](r.Body)
	if err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := validate.Struct(req); err != nil {
		httputil.ErrResponse(w, http.StatusBadRequest, err)
		return
	}

	if req.MetadataSnapshot.SnapshotVersion != 2 {
		httputil.ErrResponse(w, http.StatusBadRequest, errSnapshotVersion)
		return
	}

	ctx := r.Context()

	// Get organization from slug or return errOrgNotFound
	org, err := h.db.Queries.GetOrganizationBySlug(ctx, req.OrgSlug)
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errOrgNotFound)
		return
	}

	repo, err := h.db.Queries.GetRepoByFullNameAndOrg(ctx, store.GetRepoByFullNameAndOrgParams{
		FullName: req.RepoFullName,
		OrgID:    org.ID,
	})
	if err != nil {
		httputil.ErrResponse(w, http.StatusNotFound, errRepoNotConnected)
		return
	}

	metaJSON, err := json.Marshal(req.MetadataSnapshot)

	if err != nil {
		h.log.Error("marshal snapshot", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	gcsPath := fmt.Sprintf("%s/%s/%s/snapshot.json",
		org.ID.String(), repo.ID.String(), req.CommitSha)

	if err := h.storage.Upload(ctx, gcsPath, metaJSON); err != nil {
		h.log.Error("write snapshot to gcs", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	snap, err := h.db.Queries.InsertCLISnapshot(ctx, store.InsertCLISnapshotParams{
		RepoID:          repo.ID,
		Branch:          req.Branch,
		CommitSha:       req.CommitSha,
		GcsPath:         gcsPath,
		SnapshotVersion: 2,
		ZigzagVersion:   req.MetadataSnapshot.ZigzagVersion,
		SizeBytes:       int64(len(metaJSON)),
		Metadata:        metaJSON,
	})
	if err != nil {
		h.log.Error("insert snapshot", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, errInternal)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, uploadResponse{
		SnapshotID: snap.ID.String(),
		CreatedAt:  snap.CreatedAt.Time,
	})
}
