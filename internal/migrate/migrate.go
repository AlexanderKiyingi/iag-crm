package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This service owns the `crm` schema on the shared Railway database. The ledger
// is schema-qualified so it can never collide with another service's global
// public.schema_migrations (the collision that caused cross-service boot
// failures). db.Connect pins search_path to `crm, public`.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS crm.schema_migrations (
    version    TEXT PRIMARY KEY,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

type Migration struct {
	Version  string
	Body     string
	Checksum string
}

func Up(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) ([]string, error) {
	migs, err := load(fsys)
	if err != nil {
		return nil, err
	}

	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS crm`); err != nil {
		return nil, fmt.Errorf("create schema crm: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaMigrationsDDL); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	// One-time cutover from the shared global public.schema_migrations: stamp this
	// service's already-applied versions into the per-service ledger with their
	// current file checksums, so nothing re-runs and the mismatch/re-apply path
	// below cannot fire against tables that already exist in public.
	if err := seedFromLegacyLedger(ctx, pool, migs); err != nil {
		return nil, fmt.Errorf("seed from legacy ledger: %w", err)
	}

	applied, err := loadApplied(ctx, pool)
	if err != nil {
		return nil, err
	}

	// Base-table self-heal. A DB can carry schema_migrations rows (recorded as
	// applied) while the actual objects were never created — e.g. an earlier
	// checksum-only re-stamp marked "0001_initial" applied without running its
	// body, so its stored checksum now MATCHES the file and the mismatch branch
	// below never fires, yet crm_accounts does not exist and 0002 fails with
	// 42P01. When the base table is missing but migrations are recorded, force
	// every idempotent body to re-run so the schema is rebuilt from scratch.
	forceReapply := false
	if len(applied) > 0 {
		exists, err := baseTableExists(ctx, pool)
		if err != nil {
			return nil, err
		}
		if !exists {
			slog.Warn("schema_migrations recorded but base table crm_accounts missing; forcing idempotent re-apply of all migrations")
			forceReapply = true
		}
	}

	var newlyApplied []string
	for _, m := range migs {
		prev, ok := applied[m.Version]
		switch {
		case !ok:
			if err := apply(ctx, pool, m); err != nil {
				return newlyApplied, fmt.Errorf("migration %s: %w", m.Version, err)
			}
			newlyApplied = append(newlyApplied, m.Version)
			slog.Info("migration applied", "version", m.Version)
		case forceReapply || prev.Checksum != m.Checksum:
			// Legacy Railway DBs carry schema_migrations rows written by an
			// earlier migration tool whose body for this version differs from
			// the file shipped today — e.g. an older CRM implementation that
			// recorded "0001_initial" without ever creating crm_accounts.
			// Re-stamping the checksum alone assumes the file's objects already
			// exist; for those rows they do not, so later migrations that
			// reference them (0002 -> crm_accounts) fail with 42P01. Every
			// migration body is idempotent (CREATE ... IF NOT EXISTS, ADD
			// COLUMN IF NOT EXISTS, INSERT ... ON CONFLICT DO NOTHING), so the
			// safe self-heal is to re-run the body and then re-stamp the
			// checksum in the same transaction. Mirrors the self-heal pattern
			// used by the other services (iag-authentication 839c292).
			slog.Warn("migration checksum mismatch; re-applying idempotent body and re-stamping",
				"version", m.Version,
				"stored", prev.Checksum,
				"file", m.Checksum,
			)
			if err := reapply(ctx, pool, m); err != nil {
				return newlyApplied, fmt.Errorf(
					"migration %s re-apply: %w", m.Version, err,
				)
			}
		}
	}
	return newlyApplied, nil
}

// baseTableExists reports whether the schema's anchor table (crm_accounts,
// created by 0001_initial) is present. It resolves the name through the current
// search_path with to_regclass, matching how the migration bodies reference it.
func baseTableExists(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var reg *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('crm_accounts')::text`).Scan(&reg); err != nil {
		return false, err
	}
	return reg != nil, nil
}

// seedFromLegacyLedger stamps this service's shipped versions into crm's ledger
// using the CURRENT file checksums, for any version already recorded in a legacy
// global public.schema_migrations. Using the file checksum (rather than the
// legacy one) guarantees the checksum-mismatch re-apply path in Up does not fire
// during the shared-database cutover — those objects already exist in public and
// resolve through the search_path fallback. Idempotent; no-op on a fresh DB.
func seedFromLegacyLedger(ctx context.Context, pool *pgxpool.Pool, migs []Migration) error {
	var hasLegacy bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'schema_migrations'
		)`).Scan(&hasLegacy); err != nil {
		return err
	}
	if !hasLegacy {
		return nil
	}
	for _, m := range migs {
		if _, err := pool.Exec(ctx, `
			INSERT INTO crm.schema_migrations (version, checksum)
			SELECT $1, $2
			WHERE EXISTS (SELECT 1 FROM public.schema_migrations WHERE version = $1)
			ON CONFLICT (version) DO NOTHING`, m.Version, m.Checksum); err != nil {
			return fmt.Errorf("seed %s: %w", m.Version, err)
		}
	}
	return nil
}

func load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var out []Migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		sum := sha256.Sum256(body)
		out = append(out, Migration{
			Version:  strings.TrimSuffix(name, ".sql"),
			Body:     string(body),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

type appliedRow struct {
	Version  string
	Checksum string
}

func loadApplied(ctx context.Context, pool *pgxpool.Pool) (map[string]appliedRow, error) {
	rows, err := pool.Query(ctx, `SELECT version, checksum FROM crm.schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]appliedRow{}
	for rows.Next() {
		var r appliedRow
		if err := rows.Scan(&r.Version, &r.Checksum); err != nil {
			return nil, err
		}
		out[r.Version] = r
	}
	return out, rows.Err()
}

func apply(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.Body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO crm.schema_migrations (version, checksum) VALUES ($1, $2)`,
		m.Version, m.Checksum); err != nil {
		if strings.Contains(err.Error(), "23505") {
			return errors.New("concurrent migration: version already applied by another process")
		}
		return err
	}
	return tx.Commit(ctx)
}

// reapply re-runs an already-recorded migration whose stored checksum no longer
// matches the shipped file, then re-stamps the checksum. Safe only because every
// migration body is idempotent; running it against a DB that already has the
// objects is a no-op, while a legacy DB missing them is brought up to date.
func reapply(ctx context.Context, pool *pgxpool.Pool, m Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.Body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE crm.schema_migrations SET checksum = $1 WHERE version = $2`,
		m.Checksum, m.Version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
