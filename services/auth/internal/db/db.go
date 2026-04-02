package db

import (
	"github.com/jackc/pgx/v5/pgxpool"

	authstore "github.com/LegationPro/zagforge/auth/internal/store"
)

// DB wraps the connection pool and the sqlc-generated query interface.
type DB struct {
	Pool    *pgxpool.Pool
	Queries *authstore.Queries
}

// New creates a DB from an existing pool.
func New(pool *pgxpool.Pool) *DB {
	return &DB{Pool: pool, Queries: authstore.New(pool)}
}
