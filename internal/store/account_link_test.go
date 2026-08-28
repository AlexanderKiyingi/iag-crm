package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	crmDB "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/migrate"
)

func TestPatchedAccountName(t *testing.T) {
	if _, ok := patchedAccountName(map[string]any{"name": "unrelated"}); ok {
		t.Error("a body that sets neither key reported an account change; a plain rename would re-resolve the FK for nothing")
	}
	if name, ok := patchedAccountName(map[string]any{"account": "Alpha Ltd"}); !ok || name != "Alpha Ltd" {
		t.Errorf(`account = %q ok=%v, want "Alpha Ltd" true`, name, ok)
	}
	if name, ok := patchedAccountName(map[string]any{"account_name": "Beta Ltd"}); !ok || name != "Beta Ltd" {
		t.Errorf(`account_name = %q ok=%v, want "Beta Ltd" true`, name, ok)
	}
	// The column allowlist lets account_name win when both arrive, and the FK
	// must follow the same one or the name and the link disagree.
	both := map[string]any{"account": "Alpha Ltd", "account_name": "Beta Ltd"}
	if name, _ := patchedAccountName(both); name != "Beta Ltd" {
		t.Errorf("both keys present resolved %q, want the account_name value the column takes", name)
	}
	// An empty string is a real instruction — unlink — not an absent key.
	if name, ok := patchedAccountName(map[string]any{"account": ""}); !ok || name != "" {
		t.Errorf(`account="" = %q ok=%v, want "" true so the deal is honestly unlinked`, name, ok)
	}
	if _, ok := patchedAccountName(map[string]any{"account": 42}); ok {
		t.Error("a wrong-typed account was accepted; the column allowlist ignores it, so the FK must too")
	}
}

// Proof against real Postgres that moving a deal or a contact to another
// customer carries account_id with the display name. Before this, `account`
// wrote account_name only and the foreign key kept pointing at the previous
// account — so /accounts/:id/360 and every rollup still counted the deal under
// the old customer while the screen showed the new one.
func TestIntegration_AccountRenameRepointsForeignKey(t *testing.T) {
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
	for _, table := range []string{"crm_deals", "crm_contacts", "crm_accounts"} {
		if _, err := pool.Exec(ctx, `DELETE FROM `+table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}

	repo := New(pool)
	ids := map[string]string{}
	for _, name := range []string{"Alpha Ltd", "Beta Ltd"} {
		id := repo.NewID()
		ids[name] = id
		if _, err := pool.Exec(ctx, `
			INSERT INTO crm_accounts (id, name, account_type, country, segment, owner, value_display, status)
			VALUES ($1,$2,'customer','UG','coffee','',$3,'active')`, id, name, ""); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	deal, err := repo.CreateDeal(ctx, DealInput{Name: "Renewal", Account: "Alpha Ltd"})
	if err != nil {
		t.Fatalf("create deal: %v", err)
	}
	if deal.AccountID != ids["Alpha Ltd"] {
		t.Fatalf("new deal linked to %q, want Alpha Ltd's id", deal.AccountID)
	}

	moved, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"account": "Beta Ltd"})
	if err != nil {
		t.Fatalf("patch deal: %v", err)
	}
	if moved.Account != "Beta Ltd" {
		t.Errorf("deal account_name = %q, want Beta Ltd", moved.Account)
	}
	if moved.AccountID != ids["Beta Ltd"] {
		t.Errorf("deal account_id = %q, want Beta Ltd's id %q — the name moved and the link did not",
			moved.AccountID, ids["Beta Ltd"])
	}

	// A name matching no account unlinks rather than keeping the stale id.
	orphaned, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"account": "Nobody Ltd"})
	if err != nil {
		t.Fatalf("patch deal to unknown account: %v", err)
	}
	if orphaned.AccountID != "" {
		t.Errorf("unresolvable account left account_id = %q, want it cleared", orphaned.AccountID)
	}

	contact, err := repo.CreateContact(ctx, ContactInput{Name: "Jo Buyer", Account: "Alpha Ltd"})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	movedContact, err := repo.PatchContact(ctx, contact.ID, map[string]any{"account_name": "Beta Ltd"})
	if err != nil {
		t.Fatalf("patch contact: %v", err)
	}
	if movedContact.AccountID != ids["Beta Ltd"] {
		t.Errorf("contact account_id = %q, want Beta Ltd's id %q", movedContact.AccountID, ids["Beta Ltd"])
	}
}

// A lead nobody scored must read 0, not an invented 65 that outranks every
// honestly-scored lead below it in `ORDER BY score DESC`.
func TestIntegration_UnscoredLeadHasNoInventedScore(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `DELETE FROM crm_leads`); err != nil {
		t.Fatalf("clean: %v", err)
	}

	repo := New(pool)
	unscored, err := repo.CreateLead(ctx, LeadInput{Name: "Unscored", Email: "u@example.com"})
	if err != nil {
		t.Fatalf("create unscored: %v", err)
	}
	if unscored.Score != 0 {
		t.Errorf("unscored lead got score %d, want 0", unscored.Score)
	}

	scored, err := repo.CreateLead(ctx, LeadInput{Name: "Scored", Email: "s@example.com", Score: 40})
	if err != nil {
		t.Fatalf("create scored: %v", err)
	}
	if scored.Score != 40 {
		t.Errorf("explicit score came back as %d, want 40", scored.Score)
	}

	// The lead a human actually scored must rank above the one nobody has.
	list, _, err := repo.ListLeads(ctx, ListOpts{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].Name != "Scored" {
		t.Errorf("ranking = %+v, want the scored lead first", list)
	}
}
