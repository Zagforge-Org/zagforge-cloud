package handler

import "errors"

// Common handler errors shared across all handler packages.
var (
	ErrInternal      = errors.New("internal error")
	ErrForbidden     = errors.New("forbidden")
	ErrInvalidUserID = errors.New("invalid user id")
	ErrInvalidOrgID  = errors.New("invalid org id")
	ErrUnauthorized  = errors.New("unauthorized")
)
