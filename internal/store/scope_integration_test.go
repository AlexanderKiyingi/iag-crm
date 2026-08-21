package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	crmDB "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/migrate"
)

// End-to-end proof against real Postgres that a scoped rep cannot read another
// rep's records, and cannot escape by supplying ?owner=.
func TestIntegration_ScopeIsEnforcedInSQL(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if _, err := migrate.Up(ctx, pool, crmDB.Migrations()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM crm_accounts`); err != nil {
		t.Fatalf("clean: %v", err)
	}

	repo := New(pool)
	for _, o := range []struct{ id, name, owner string }{
		{"ACC-R1", "Rep One Co", "rep@iag.com"},
		{"ACC-R2", "Rep Two Co", "other@iag.com"},
		{"ACC-R3", "Director Co", "director@iag.com"},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO crm_accounts (id, name, account_type, country, segment, owner, value_display, status)
			VALUES ($1,$2,'customer','UG','coffee',$3,'',$4)`, o.id, o.name, o.owner, "active"); err != nil {
			t.Fatalf("seed %s: %v", o.id, err)
		}
	}

	// Unscoped caller (manager) sees everything.
	all, total, err := repo.ListAccounts(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("list unscoped: %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("unscoped saw %d/%d, want 3", len(all), total)
	}

	// Scoped rep sees only their own.
	mine, total, err := repo.ListAccounts(ctx, ListOpts{ScopeOwner: "rep@iag.com"})
	if err != nil {
		t.Fatalf("list scoped: %v", err)
	}
	if total != 1 || len(mine) != 1 || mine[0].ID != "ACC-R1" {
		t.Fatalf("scoped rep saw %d rows (%+v), want only ACC-R1", len(mine), mine)
	}

	// THE BYPASS: rep supplies ?owner=director@iag.com. Must return nothing,
	// not the director's book.
	escaped, total, err := repo.ListAccounts(ctx, ListOpts{
		ScopeOwner: "rep@iag.com",
		Owner:      "director@iag.com",
	})
	if err != nil {
		t.Fatalf("list with bypass attempt: %v", err)
	}
	if total != 0 || len(escaped) != 0 {
		t.Fatalf("BYPASS: rep read %d of another owner's accounts (%+v)", len(escaped), escaped)
	}
}
