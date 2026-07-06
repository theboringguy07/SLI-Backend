// Package repositories talks to Postgres directly through pgx - no ORM.
// Every repository takes a dbtx (satisfied by both *pgxpool.Pool and pgx.Tx)
// so the two repositories that need transactions (report, evaluation) can
// swap in a transaction-scoped instance of themselves without a second
// implementation; see reportRepository.RunInTransaction for the pattern.
package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// dbtx is the common subset of *pgxpool.Pool and pgx.Tx that repositories
// need. Any function that would otherwise take *pgxpool.Pool takes this
// instead, so it also accepts a transaction.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// txBeginner is implemented by both *pgxpool.Pool (starts a real
// transaction) and pgx.Tx (opens a SAVEPOINT - Postgres's take on nested
// transactions; Commit/Rollback on it become RELEASE/ROLLBACK TO SAVEPOINT).
// RunInTransaction type-asserts for this rather than assuming r.db is
// specifically a pool, so it works whether or not it's already nested inside
// another transaction.
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
