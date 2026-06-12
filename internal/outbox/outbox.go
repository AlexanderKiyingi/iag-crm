// Package outbox implements the transactional outbox pattern for CRM.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Row is a pending or completed outbox entry.
type Row struct {
	ID          int64
	EventType   string
	EventKey    string
	Payload     json.RawMessage
	CreatedAt   time.Time
	AvailableAt time.Time
	Attempts    int
	LastError   string
}

// Store wraps the crm_event_outbox table.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Enqueue(ctx context.Context, eventType, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	exec := txExecOr(ctx, s.pool)
	_, err = exec.Exec(ctx, `
		INSERT INTO crm_event_outbox (event_type, event_key, payload)
		VALUES ($1, $2, $3::jsonb)
	`, eventType, nullable(key), body)
	return err
}

func (s *Store) ClaimBatch(ctx context.Context, limit int, backoff time.Duration) ([]Row, error) {
	if limit <= 0 {
		limit = 32
	}
	rows, err := s.pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM crm_event_outbox
			WHERE dispatched_at IS NULL AND abandoned_at IS NULL AND available_at <= NOW()
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE crm_event_outbox o
		SET attempts = o.attempts + 1,
		    available_at = NOW() + $2::interval
		FROM due
		WHERE o.id = due.id
		RETURNING o.id, o.event_type, o.event_key, o.payload, o.created_at,
		          o.available_at, o.attempts, COALESCE(o.last_error, '')
	`, limit, fmt.Sprintf("%d milliseconds", backoff.Milliseconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Row{}
	for rows.Next() {
		var r Row
		var key *string
		if err := rows.Scan(&r.ID, &r.EventType, &key, &r.Payload, &r.CreatedAt,
			&r.AvailableAt, &r.Attempts, &r.LastError); err != nil {
			return nil, err
		}
		if key != nil {
			r.EventKey = *key
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) MarkDispatched(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crm_event_outbox
		SET dispatched_at = NOW(), last_error = NULL
		WHERE id = $1
	`, id)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id int64, errMsg string, retryDelay time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crm_event_outbox
		SET last_error = $1, available_at = NOW() + $2::interval
		WHERE id = $3
	`, errMsg, fmt.Sprintf("%d milliseconds", retryDelay.Milliseconds()), id)
	return err
}

// MarkAbandoned moves a row to the dead-letter state after it exhausts its retry
// budget, so the publisher stops retrying it. The row is retained (not deleted)
// with its last error for inspection.
func (s *Store) MarkAbandoned(ctx context.Context, id int64, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crm_event_outbox
		SET abandoned_at = NOW(), last_error = $1
		WHERE id = $2
	`, errMsg, id)
	return err
}

// Dispatcher writes a drained outbox row to Kafka.
type Dispatcher interface {
	DispatchOutbox(ctx context.Context, row Row) error
}

// Publisher periodically drains the outbox.
type Publisher struct {
	store       *Store
	dispatcher  Dispatcher
	tick        time.Duration
	batch       int
	maxBackoff  time.Duration
	maxAttempts int
}

func NewPublisher(store *Store, d Dispatcher) *Publisher {
	return &Publisher{
		store:       store,
		dispatcher:  d,
		tick:        2 * time.Second,
		batch:       32,
		maxBackoff:  5 * time.Minute,
		maxAttempts: 12,
	}
}

func (p *Publisher) Run(ctx context.Context) {
	if p == nil || p.store == nil || p.dispatcher == nil {
		return
	}
	ticker := time.NewTicker(p.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := p.drainOnce(ctx)
			if err != nil {
				slog.Warn("outbox drain", "err", err)
				continue
			}
			if n >= p.batch {
				if _, err := p.drainOnce(ctx); err != nil {
					slog.Warn("outbox follow-up drain", "err", err)
				}
			}
		}
	}
}

func (p *Publisher) drainOnce(ctx context.Context) (int, error) {
	rows, err := p.store.ClaimBatch(ctx, p.batch, time.Second)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		if err := p.dispatcher.DispatchOutbox(ctx, r); err != nil {
			if r.Attempts >= p.maxAttempts {
				if mErr := p.store.MarkAbandoned(ctx, r.ID, err.Error()); mErr != nil {
					slog.Warn("outbox mark-abandoned", "id", r.ID, "err", mErr)
				}
				slog.Error("outbox event abandoned after max attempts", "id", r.ID,
					"type", r.EventType, "attempts", r.Attempts, "maxAttempts", p.maxAttempts, "err", err)
				continue
			}
			delay := backoffFor(r.Attempts, p.maxBackoff)
			if mErr := p.store.MarkFailed(ctx, r.ID, err.Error(), delay); mErr != nil {
				slog.Warn("outbox mark-failed", "id", r.ID, "err", mErr)
			}
			slog.Warn("outbox dispatch failed", "id", r.ID, "type", r.EventType,
				"attempts", r.Attempts, "err", err, "retryIn", delay)
			continue
		}
		if mErr := p.store.MarkDispatched(ctx, r.ID); mErr != nil {
			slog.Warn("outbox mark-dispatched", "id", r.ID, "err", mErr)
		}
	}
	return len(rows), nil
}

func backoffFor(attempts int, max time.Duration) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := time.Duration(math.Pow(2, float64(attempts))) * time.Second
	if d > max {
		return max
	}
	return d
}

type txKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txExecOr(ctx context.Context, pool *pgxpool.Pool) execer {
	if tx, ok := TxFrom(ctx); ok {
		return tx
	}
	return pool
}

// TxFrom returns the transaction stored in ctx by WithTx, if any. It lets other
// packages (e.g. the store's unit-of-work helper) share the same transaction so
// a domain write and its outbox enqueue commit atomically.
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
	if v := ctx.Value(txKey{}); v != nil {
		if tx, ok := v.(pgx.Tx); ok && tx != nil {
			return tx, true
		}
	}
	return nil, false
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var ErrNotEnqueued = errors.New("outbox: publisher not configured")
