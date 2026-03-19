package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge-mvp-impl/shared/go/logger"
	githubprovider "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
	"github.com/LegationPro/zagforge-mvp-impl/shared/go/runner"
	"github.com/LegationPro/zagforge-mvp-impl/shared/go/store"
)

const pollInterval = 2 * time.Second

func run() error {
	log, err := logger.New(os.Getenv("APP_ENV"))
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer log.Sync()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer pool.Close()

	queries := store.New(pool)

	ghAppID := os.Getenv("GITHUB_APP_ID")
	ghKey := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	ghSecret := os.Getenv("GITHUB_APP_WEBHOOK_SECRET")

	// Parse app ID.
	var appID int64
	if _, err := fmt.Sscanf(ghAppID, "%d", &appID); err != nil {
		return fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}

	client, err := githubprovider.NewAPIClient(appID, []byte(ghKey), ghSecret)
	if err != nil {
		return fmt.Errorf("create API client: %w", err)
	}

	ch, err := githubprovider.NewClientHandler(client)
	if err != nil {
		return fmt.Errorf("create client handler: %w", err)
	}

	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = filepath.Join(os.TempDir(), "zagforge-workspace")
	}
	zigzagBin := os.Getenv("ZIGZAG_BIN")
	if zigzagBin == "" {
		zigzagBin = "zagforge-worker"
	}
	reportsDir := os.Getenv("REPORTS_DIR")
	if reportsDir == "" {
		reportsDir = "/data/reports"
	}

	r := runner.New(ch, runner.Config{
		WorkspaceDir: workspaceDir,
		ZigzagBin:    zigzagBin,
		ReportsDir:   reportsDir,
	}, log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("worker started, polling for jobs")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down worker", zap.Int64("in_flight_jobs", r.InFlight()))
			if err := r.Drain(2*time.Minute, 5*time.Second); err != nil {
				return err
			}
			log.Info("worker stopped")
			return nil
		case <-ticker.C:
			if err := pollOnce(ctx, queries, r, log); err != nil {
				log.Error("poll error", zap.Error(err))
			}
		}
	}
}

func pollOnce(ctx context.Context, queries *store.Queries, r *runner.Runner, log *zap.Logger) error {
	job, err := queries.ClaimJob(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return fmt.Errorf("claim job: %w", err)
	}

	repo, err := queries.GetRepoForJob(ctx, job.ID)
	if err != nil {
		queries.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
			ID:           job.ID,
			Status:       store.JobStatusFailed,
			ErrorMessage: pgtype.Text{String: "repo not found for job", Valid: true},
		})
		return fmt.Errorf("get repo for job: %w", err)
	}

	log.Info("claimed job",
		zap.String("job_id", job.ID.String()),
		zap.String("repo", repo.FullName),
		zap.String("branch", job.Branch),
		zap.String("commit", job.CommitSha),
	)

	r.GoWait(func() {
		executeJob(context.Background(), queries, r, log, job, repo)
	})

	return nil
}

func executeJob(ctx context.Context, queries *store.Queries, r *runner.Runner, log *zap.Logger, job store.Job, repo store.GetRepoForJobRow) {
	cloneURL := fmt.Sprintf("https://github.com/%s.git", repo.FullName)

	result, err := r.Run(ctx, githubprovider.WebhookEvent{
		RepoID:         repo.GithubRepoID,
		RepoName:       repo.FullName,
		CloneURL:       cloneURL,
		Branch:         job.Branch,
		CommitSHA:      job.CommitSha,
		InstallationID: repo.InstallationID,
	})
	if err != nil {
		log.Error("job failed",
			zap.String("job_id", job.ID.String()),
			zap.String("repo", repo.FullName),
			zap.Error(err),
		)
		queries.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
			ID:           job.ID,
			Status:       store.JobStatusFailed,
			ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
		})
		return
	}

	_, snapErr := queries.InsertSnapshot(ctx, store.InsertSnapshotParams{
		RepoID:          job.RepoID,
		JobID:           job.ID,
		Branch:          job.Branch,
		CommitSha:       job.CommitSha,
		GcsPath:         result.ReportsDir,
		SnapshotVersion: 1,
		ZigzagVersion:   result.ZigzagVersion,
		SizeBytes:       result.SizeBytes,
	})
	if snapErr != nil {
		log.Error("failed to insert snapshot", zap.String("job_id", job.ID.String()), zap.Error(snapErr))
	}

	queries.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
		ID:     job.ID,
		Status: store.JobStatusSucceeded,
	})

	log.Info("job succeeded",
		zap.String("job_id", job.ID.String()),
		zap.String("repo", repo.FullName),
		zap.String("zigzag_version", result.ZigzagVersion),
		zap.Int64("size_bytes", result.SizeBytes),
	)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
