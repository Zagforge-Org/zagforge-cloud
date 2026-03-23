//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge/api/internal/cache/contextcache"
	dbpkg "github.com/LegationPro/zagforge/api/internal/db"
	apihandler "github.com/LegationPro/zagforge/api/internal/handler/api"
	contexturlhandler "github.com/LegationPro/zagforge/api/internal/handler/contexturl"
	"github.com/LegationPro/zagforge/api/internal/handler/health"
	uploadhandler "github.com/LegationPro/zagforge/api/internal/handler/upload"
	"github.com/LegationPro/zagforge/api/internal/handler/watchdog"
	"github.com/LegationPro/zagforge/api/internal/middleware/clitoken"
	"github.com/LegationPro/zagforge/api/internal/middleware/contenttype"
	corsmw "github.com/LegationPro/zagforge/api/internal/middleware/cors"
	"github.com/LegationPro/zagforge/shared/go/httputil"
	"github.com/LegationPro/zagforge/shared/go/router"
	storagepkg "github.com/LegationPro/zagforge/shared/go/storage"
	"github.com/LegationPro/zagforge/shared/go/store"
)

const testCLIAPIKey = "zf_test_cli_key"

type testEnv struct {
	server  *httptest.Server
	db      *dbpkg.DB
	pool    *pgxpool.Pool
	storage *storagepkg.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://zagforge:zagforge@localhost:5432/zagforge_test?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	database := dbpkg.New(pool)
	log := zap.NewNop()

	// GCS client (fake-gcs-server).
	gcsEndpoint := os.Getenv("TEST_GCS_ENDPOINT")
	if gcsEndpoint == "" {
		gcsEndpoint = "http://localhost:4443/storage/v1/"
	}
	gcsBucket := os.Getenv("TEST_GCS_BUCKET")
	if gcsBucket == "" {
		gcsBucket = "zagforge-snapshots-test"
	}
	// Ensure the test bucket exists in fake-gcs-server.
	createBucketURL := fmt.Sprintf("%sb?project=test", gcsEndpoint)
	bucketBody := fmt.Sprintf(`{"name":%q}`, gcsBucket)
	bucketReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, createBucketURL, bytes.NewBufferString(bucketBody))
	bucketReq.Header.Set("Content-Type", "application/json")
	if resp, err := http.DefaultClient.Do(bucketReq); err == nil {
		_ = resp.Body.Close()
	}

	gcsClient, err := storagepkg.NewClient(ctx, storagepkg.Config{
		Bucket:   gcsBucket,
		Endpoint: gcsEndpoint,
	}, log)
	if err != nil {
		t.Fatalf("create gcs client: %v", err)
	}

	ctxCache := contextcache.NewInMemory()

	r := router.New()
	r.Use(corsmw.Cors([]string{"http://localhost:3000"}))

	healthH := health.NewHandler(pool)
	apiH := apihandler.NewHandler(database, log)
	watchdogH := watchdog.NewHandler(database, log)
	uploadH := uploadhandler.NewHandler(database, gcsClient, log)
	contextURLH := contexturlhandler.NewHandler(database, ctxCache, nil, gcsClient, log)

	healthRoutes := r.Group()
	_ = healthRoutes.Create([]router.Subroute{
		{Method: router.GET, Path: "/livez", Handler: healthH.Liveness},
		{Method: router.GET, Path: "/readyz", Handler: healthH.Readiness},
	})

	watchdogRoutes := r.Group()
	_ = watchdogRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/internal/watchdog/timeout", Handler: watchdogH.Timeout},
	})

	v1 := r.Group()
	_ = v1.Create([]router.Subroute{
		{Method: router.GET, Path: "/api/v1/repos/{repoID}", Handler: apiH.GetRepo},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs", Handler: apiH.ListJobs},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/jobs/{jobID}", Handler: apiH.GetJob},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots", Handler: apiH.ListSnapshots},
		{Method: router.GET, Path: "/api/v1/repos/{repoID}/snapshots/latest", Handler: apiH.GetLatestSnapshot},
		{Method: router.GET, Path: "/api/v1/snapshots/{snapshotID}", Handler: apiH.GetSnapshot},
	})

	// Context URL — public, no auth.
	contextRoutes := r.Group()
	_ = contextRoutes.Create([]router.Subroute{
		{Method: router.HEAD, Path: "/v1/context/{token}", Handler: contextURLH.Head},
		{Method: router.GET, Path: "/v1/context/{token}", Handler: contextURLH.Get},
	})

	// CLI upload — CLI token auth.
	uploadRoutes := r.Group()
	uploadRoutes.Use(contenttype.RequireJSON())
	uploadRoutes.Use(clitoken.Auth(testCLIAPIKey))
	_ = uploadRoutes.Create([]router.Subroute{
		{Method: router.POST, Path: "/api/v1/upload", Handler: uploadH.Upload},
	})

	server := httptest.NewServer(r.Handler())

	env := &testEnv{server: server, db: database, pool: pool, storage: gcsClient}

	t.Cleanup(func() {
		server.Close()
		pool.Close()
	})

	return env
}

func (e *testEnv) seed(t *testing.T) (orgID, repoID string) {
	t.Helper()
	ctx := context.Background()

	org, err := e.db.Queries.UpsertOrg(ctx, store.UpsertOrgParams{
		ClerkOrgID: fmt.Sprintf("test_org_%d", time.Now().UnixNano()),
		Slug:       fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Name:       "Test Org",
	})
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	repo, err := e.db.Queries.UpsertRepo(ctx, store.UpsertRepoParams{
		OrgID:          org.ID,
		GithubRepoID:   time.Now().UnixNano(),
		InstallationID: 12345,
		FullName:       fmt.Sprintf("test-org/test-repo-%d", time.Now().UnixNano()),
		DefaultBranch:  "main",
	})
	if err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	return org.ID.String(), repo.ID.String()
}

// seedWithNames seeds an org and repo with specific slug and full_name for upload tests.
func (e *testEnv) seedWithNames(t *testing.T, orgSlug, repoFullName string) (orgID, repoID string) {
	t.Helper()
	ctx := context.Background()

	org, err := e.db.Queries.UpsertOrg(ctx, store.UpsertOrgParams{
		ClerkOrgID: fmt.Sprintf("test_org_%d", time.Now().UnixNano()),
		Slug:       orgSlug,
		Name:       "Test Org",
	})
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	repo, err := e.db.Queries.UpsertRepo(ctx, store.UpsertRepoParams{
		OrgID:          org.ID,
		GithubRepoID:   time.Now().UnixNano(),
		InstallationID: 12345,
		FullName:       repoFullName,
		DefaultBranch:  "main",
	})
	if err != nil {
		t.Fatalf("seed repo: %v", err)
	}

	return org.ID.String(), repo.ID.String()
}

func (e *testEnv) createJob(t *testing.T, repoID, branch, commitSHA string) string {
	t.Helper()
	ctx := context.Background()

	repoUUID, err := httputil.UUIDFromString(repoID)
	if err != nil {
		t.Fatalf("parse repo id: %v", err)
	}

	job, err := e.db.Queries.CreateJob(ctx, store.CreateJobParams{
		RepoID:    repoUUID,
		Branch:    branch,
		CommitSha: commitSHA,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	return job.ID.String()
}

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(e.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) head(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := http.Head(e.server.URL + path)
	if err != nil {
		t.Fatalf("HEAD %s: %v", path, err)
	}
	return resp
}

func (e *testEnv) postJSON(t *testing.T, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}
