package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	crmdb "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/db"
)

// dial builds the pool the way the service does, so the test resolves unqualified
// names through the same search_path (`crm, public`) on every pooled connection.
// Setting it with a one-off Exec would pin it to whichever connection served that
// call and leave the rest of the pool reading `public` — which quietly turned a
// dropped crm.crm_accounts into a pass, because a stale public.crm_accounts still
// answered to_regclass.
func dial(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CRM_TEST_DSN")
	if dsn == "" {
		t.Skip("CRM_TEST_DSN not set")
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestChecksumHealLive is the regression for the Railway crash loop: the boot
// logged "migration checksum mismatch; re-applying idempotent body and
// re-stamping" on every start and then exited, because the re-apply of
// 0011_uuid_entity_ids threw against an already-converted database, rolled its
// transaction back, and left the mismatching checksum in place to be found again
// on the next boot.
//
// Point CRM_TEST_DSN at a scratch database (it must be allowed to CREATE SCHEMA):
//
//	CRM_TEST_DSN='postgres://iag:iag_dev@localhost:5432/iag_platform?sslmode=disable' go test ./internal/migrate/
func TestChecksumHealLive(t *testing.T) {
	ctx := context.Background()
	pool := dial(ctx, t)

	if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
		t.Fatalf("initial Up: %v", err)
	}
	if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
		t.Fatalf("second Up must be a no-op, got: %v", err)
	}

	migs, err := load(crmdb.Migrations())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Every body has to survive the self-heal, so drive all of them through it.
	// A single non-idempotent migration is enough to brick the service.
	for _, m := range migs {
		if _, err := pool.Exec(ctx,
			`UPDATE crm.schema_migrations SET checksum = 'deadbeef' WHERE version = $1`,
			m.Version); err != nil {
			t.Fatalf("stamp bogus checksum on %s: %v", m.Version, err)
		}
		if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
			t.Fatalf("re-apply of %s must heal, got: %v", m.Version, err)
		}
		var stored string
		if err := pool.QueryRow(ctx,
			`SELECT checksum FROM crm.schema_migrations WHERE version = $1`,
			m.Version).Scan(&stored); err != nil {
			t.Fatalf("read checksum for %s: %v", m.Version, err)
		}
		if stored != m.Checksum {
			t.Fatalf("%s was not re-stamped: stored %s, file %s", m.Version, stored, m.Checksum)
		}
	}

	// A ledger written from a Windows checkout recorded CRLF checksums. Those
	// must be adopted silently rather than driving a re-apply of everything.
	for _, m := range migs {
		if _, err := pool.Exec(ctx,
			`UPDATE crm.schema_migrations SET checksum = $1 WHERE version = $2`,
			m.EOLChecksum, m.Version); err != nil {
			t.Fatalf("stamp line-ending checksum on %s: %v", m.Version, err)
		}
	}
	if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
		t.Fatalf("line-ending drift must heal, got: %v", err)
	}
	var drifted int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM crm.schema_migrations WHERE checksum = ANY($1)`,
		eolChecksums(migs)).Scan(&drifted); err != nil {
		t.Fatalf("count drifted: %v", err)
	}
	if drifted != 0 {
		t.Fatalf("%d migrations still carry a line-ending checksum", drifted)
	}
}

// TestBaseTableHealLive covers the other route into the re-apply path: the
// ledger says everything ran but the schema's anchor table is gone, so every
// body is re-run to rebuild it.
func TestBaseTableHealLive(t *testing.T) {
	ctx := context.Background()
	pool := dial(ctx, t)
	if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
		t.Fatalf("initial Up: %v", err)
	}

	before := foreignKeys(ctx, t, pool)

	// CASCADE takes the account foreign keys with it, so a rebuild that only
	// re-creates the table leaves the schema quietly weaker than it was.
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS crm.crm_accounts CASCADE`); err != nil {
		t.Fatalf("drop base table: %v", err)
	}
	if _, err := Up(ctx, pool, crmdb.Migrations()); err != nil {
		t.Fatalf("base-table self-heal must rebuild the schema, got: %v", err)
	}
	exists, err := baseTableExists(ctx, pool)
	if err != nil {
		t.Fatalf("base table check: %v", err)
	}
	if !exists {
		t.Fatal("crm_accounts was not rebuilt")
	}
	var idType string
	if err := pool.QueryRow(ctx, `
		SELECT format_type(atttypid, atttypmod) FROM pg_attribute
		 WHERE attrelid = to_regclass('crm_accounts') AND attname = 'id'`).Scan(&idType); err != nil {
		t.Fatalf("read id type: %v", err)
	}
	if idType != "uuid" {
		t.Fatalf("rebuilt crm_accounts.id is %s, want uuid", idType)
	}
	if after := foreignKeys(ctx, t, pool); len(after) != len(before) {
		t.Fatalf("foreign keys not restored: had %d, now %d (missing %v)",
			len(before), len(after), missing(before, after))
	}
}

func foreignKeys(ctx context.Context, t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT c.conrelid::regclass::text || '.' || c.conname
		  FROM pg_constraint c
		  JOIN pg_class t ON t.oid = c.conrelid
		  JOIN pg_namespace n ON n.oid = t.relnamespace
		 WHERE n.nspname = 'crm' AND c.contype = 'f'
		 ORDER BY 1`)
	if err != nil {
		t.Fatalf("list foreign keys: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan foreign key: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list foreign keys: %v", err)
	}
	return out
}

func missing(before, after []string) []string {
	have := map[string]bool{}
	for _, s := range after {
		have[s] = true
	}
	var out []string
	for _, s := range before {
		if !have[s] {
			out = append(out, s)
		}
	}
	return out
}

func eolChecksums(migs []Migration) []string {
	out := make([]string, 0, len(migs))
	for _, m := range migs {
		out = append(out, m.EOLChecksum)
	}
	return out
}
