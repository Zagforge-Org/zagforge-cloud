package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	dbpkg "github.com/LegationPro/zagforge-mvp-impl/api/internal/db"
	dbsqlc "github.com/LegationPro/zagforge-mvp-impl/api/internal/db/sqlc"
	"github.com/LegationPro/zagforge-mvp-impl/api/internal/runner"
	github "github.com/LegationPro/zagforge-mvp-impl/shared/go/provider/github"
)

var _ dispatcher = (*runner.Runner)(nil)

// dispatcher is satisfied by *runner.Runner.
type dispatcher interface {
	Dispatch(ctx context.Context, event github.WebhookEvent)
}

// JobService orchestrates job creation with deduplication.
// It satisfies handler.pushHandler.
type JobService struct {
	db  *dbpkg.DB
	run dispatcher
}

func NewJobService(db *dbpkg.DB, run dispatcher) *JobService {
	return &JobService{db: db, run: run}
}

// HandlePush persists a new queued job for the push event (with dedup) then dispatches it.
// If the repo is not registered, the event is silently dropped.
// If deliveryID is empty (header absent), it is stored as NULL.
func (s *JobService) HandlePush(ctx context.Context, event github.WebhookEvent, deliveryID string) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if err := tx.Rollback(context.Background()); err != nil && err != sql.ErrTxDone {
			log.Printf("job service: rollback error: %v", err)
		}
	}()

	qtx := dbsqlc.New(tx)

	// 1. Look up registered repo — drop silently if not found.
	repo, err := qtx.GetRepoByGithubID(ctx, event.RepoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get repo: %w", err)
	}

	// 2. Acquire per-(repo, branch) advisory lock for the duration of this transaction.
	// hashtext returns int4, cast to int8 for pg_advisory_xact_lock.
	if _, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1::text || ':' || $2)::bigint)",
		repo.ID, event.Branch,
	); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}

	// 3. Supersede any existing queued jobs for this branch.
	active, err := qtx.GetActiveJobsForBranch(ctx, dbsqlc.GetActiveJobsForBranchParams{
		RepoID: repo.ID,
		Branch: event.Branch,
	})
	if err != nil {
		return fmt.Errorf("get active jobs: %w", err)
	}
	for _, j := range active {
		if j.Status == dbsqlc.JobStatusQueued {
			if err := qtx.MarkJobSuperseded(ctx, j.ID); err != nil {
				return fmt.Errorf("mark job superseded: %w", err)
			}
		}
	}

	// 4. Insert new queued job. NULLIF in the SQL converts empty deliveryID to NULL.
	if _, err := qtx.CreateJob(ctx, dbsqlc.CreateJobParams{
		RepoID:    repo.ID,
		Branch:    event.Branch,
		CommitSha: event.CommitSHA,
		Column4:   deliveryID,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil // duplicate delivery, no-op
		}
		return fmt.Errorf("create job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// 5. Dispatch outside the transaction with detached context.
	s.run.Dispatch(context.Background(), event)
	return nil
}
