// Package database is the API's access to Postgres: the interface queries run
// against, and the migrations the binary applies at startup (ADR 0004).
package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is what queries need from the database. Both *pgxpool.Pool and pgx.Tx
// satisfy it, so the same code runs on the pool in production and inside a
// rolled-back transaction in tests. Begin on a pgx.Tx opens a savepoint.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}
