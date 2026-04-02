package httputil

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToString converts a pgtype.UUID to its string representation.
// Returns an empty string if the UUID is not valid.
func UUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}
