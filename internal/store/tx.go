package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iag/crm/backend/internal/outbox"
)

// queryer is the subset of pgx methods shared by *pgxpool.Pool and pgx.Tx, so a
// store method can run against either the pool or an in-flight transaction.
type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// db returns the transaction carried by ctx (set by WithinTx) if present,
// otherwise the connection pool. Methods that use r.db(ctx) instead of r.pool
// transparently join a caller's transaction; with no transaction in ctx the
// behaviour is identical to using the pool directly.
func (r *Repository) db(ctx context.Context) queryer {
	if tx, ok := outbox.TxFrom(ctx); ok {
		return tx
	}
	return r.pool
}

// WithinTx runs fn inside a single database transaction. The transaction is
// stowed in the context (shared with the outbox via outbox.WithTx), so domain
// writes and event enqueues performed through r.db(ctx) / the events bus commit
// atomically. If a transaction is already active on ctx, fn is run on it
// directly (nested calls reuse the outer transaction).
func (r *Repository) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := outbox.TxFrom(ctx); ok {
		return fn(ctx)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(outbox.WithTx(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
