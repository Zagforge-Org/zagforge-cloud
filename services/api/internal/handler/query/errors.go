package query

import (
	"errors"

	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
)

var (
	errInvalidBody      = handlerpkg.ErrInvalidBody
	errRepoNotFound     = errors.New("repository not found")
	errNoAIKey          = errors.New("no AI provider key configured — add one in Settings")
	errSnapshotNotFound = errors.New("no snapshot available")
	errSnapshotOutdated = errors.New("snapshot outdated: re-run zigzag --upload to generate a v2 snapshot")
	errInternal         = handlerpkg.ErrInternal
)
