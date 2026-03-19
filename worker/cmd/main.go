package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/LegationPro/zagforge-mvp-impl/api/internal/config"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/db"
	dbsqlc "github.com/LegationPro/zagforge-mvp-impl/api/internal/db/sqlc"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/runner"
	"github.com/LegationPro/zagforge-mvp-impl/shared/go/logger"
	githubprovider "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
)

const pollInterval = 2 * time.Second

func run() error {
	c, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(os.Getenv("APP_ENV"))
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer log.Sync()

	pool, err := db.Connect(context.Background(), c.DB.URL)
	if err != nil {
		return fmt.Errorf("connect to db: %w", err)
	}
	defer pool.Close()

	database := db.New(pool)

	client, err := githubprovider.NewAPIClient(c.App.GithubAppID, []byte(c.App.GithubAppPrivateKey), c.App.GithubAppWebhookSecret)
	if err != nil {
		return fmt.Errorf("create API client: %w", err)
	}

	ch, err := githubprovider.NewClientHandler(client)
	if err != nil {
		return fmt.Errorf("create client handler: %w", err)
	}

	r := runner.New(ch, runner.Config{
		WorkspaceDir: c.Worker.WorkspaceDir,
		ZigzagBin:    c.Worker.ZigzagBin,
		ReportsDir:   c.Worker.ReportsDir,
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
			if err := pollOnce(ctx, database, r, log); err != nil {
				log.Error("poll error", zap.Error(err))
			}
		}
	}
}

// pollOnce tries to claim one queued job and execute it.
func pollOnce(ctx context.Context, database *db.DB, r *runner.Runner, log *zap.Logger) error {
	job, err := database.Queries.ClaimJob(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil // nothing to do
		}
		return fmt.Errorf("claim job: %w", err)
	}

	repo, err := database.Queries.GetRepoForJob(ctx, job.ID)
	if err != nil {
		// Job claimed but repo missing — mark failed.
		database.Queries.UpdateJobStatus(ctx, dbsqlc.UpdateJobStatusParams{
			ID:           job.ID,
			Status:       dbsqlc.JobStatusFailed,
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

	// Run in a tracked goroutine so Drain can wait for it.
	r.GoWait(func() {
		executeJob(context.Background(), database, r, log, job, repo)
	})

	return nil
}

func executeJob(ctx context.Context, database *db.DB, r *runner.Runner, log *zap.Logger, job dbsqlc.Job, repo dbsqlc.GetRepoForJobRow) {
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
		database.Queries.UpdateJobStatus(ctx, dbsqlc.UpdateJobStatusParams{
			ID:           job.ID,
			Status:       dbsqlc.JobStatusFailed,
			ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
		})
		return
	}

	// Insert snapshot.
	_, snapErr := database.Queries.InsertSnapshot(ctx, dbsqlc.InsertSnapshotParams{
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

	// Mark succeeded.
	database.Queries.UpdateJobStatus(ctx, dbsqlc.UpdateJobStatusParams{
		ID:     job.ID,
		Status: dbsqlc.JobStatusSucceeded,
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
