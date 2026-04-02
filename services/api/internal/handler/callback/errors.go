package callback

import (
	"errors"

	handlerpkg "github.com/LegationPro/zagforge/api/internal/handler"
)

var (
	ErrInvalidRequestBody = handlerpkg.ErrInvalidBody
	ErrJobIDMismatch      = errors.New("job_id mismatch")
	ErrInvalidJobID       = errors.New("invalid job_id")
	ErrJobNotFound        = errors.New("job not found")
	ErrJobAlreadyTerminal = errors.New("job already in terminal state")
	ErrInternal           = handlerpkg.ErrInternal
	ErrFailedToCloneToken = errors.New("failed to generate clone token")
)
