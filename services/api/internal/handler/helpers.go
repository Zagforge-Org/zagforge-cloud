package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/LegationPro/zagforge/shared/go/store"
)

// VerifyRepoOwnership checks that the repo exists and belongs to the given org.
// Returns ErrNotFound if the repo does not exist or belongs to a different org.
func VerifyRepoOwnership(ctx context.Context, queries *store.Queries, repoID, orgID pgtype.UUID) error {
	repo, err := queries.GetRepoByID(ctx, repoID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if repo.OrgID != orgID {
		return ErrNotFound
	}
	return nil
}

// SHA256Hash returns the hex-encoded SHA-256 hash of s.
func SHA256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
