package handler

import "errors"

var (
	ErrInternal    = errors.New("internal error")
	ErrInvalidBody = errors.New("invalid request body")
	ErrNotFound    = errors.New("not found")
	ErrForbidden   = errors.New("forbidden")
)
