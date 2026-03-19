package db

import (
	"github.com/jackc/pgx/v5/pgxpool"

	dbsqlc "github.com/LegationPro/zagforge-mvp-impl/api/internal/db/sqlc"
)

// DB wraps the connection pool and the sqlc-generated query interface.
type DB struct {
	Pool    *pgxpool.Pool
	Queries *dbsqlc.Queries
}

// New creates a DB from an existing pool.
func New(pool *pgxpool.Pool) *DB {
	return &DB{Pool: pool, Queries: dbsqlc.New(pool)}
}
