package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	crmDB "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/migrate"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := migrate.Up(ctx, pool, crmDB.Migrations()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// Every field below was accepted with a 200 and then silently dropped while the
// 0008 columns were unread. These assert the round trip that proves otherwise:
// write it, read it back off a fresh query, and see the value.
//
// Deliberately a round trip rather than a struct-tag reading — the whole class
// of bug here was a payload the service accepted and did not store, which only
// a read-back can catch.

func TestIntegration_LeadAttrsRoundTrip(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_leads`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	lead, err := repo.CreateLead(ctx, LeadInput{
		Name:  "Attrs Lead",
		Email: "attrs@example.com",
		Attrs: map[string]any{
			"stage":          "Qualification",
			"estimatedValue": "4200",
			"currency":       "UGX",
			"nextFollowUp":   "2026-09-15",
			"customer":       "Alpha Ltd",
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if lead.Attrs["stage"] != "Qualification" || lead.Attrs["customer"] != "Alpha Ltd" {
		t.Fatalf("create dropped attrs: %+v", lead.Attrs)
	}

	// Read back through a separate query — the create returns its own re-read,
	// so agreeing with itself proves nothing about what landed in the column.
	fetched, err := repo.GetLead(ctx, lead.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Attrs["estimatedValue"] != "4200" || fetched.Attrs["nextFollowUp"] != "2026-09-15" {
		t.Errorf("attrs did not survive the write: %+v", fetched.Attrs)
	}

	// Replace, not merge: a key the operator removed must not come back.
	patched, err := repo.PatchLead(ctx, lead.ID, map[string]any{
		"attrs": map[string]any{"stage": "Proposal"},
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if patched.Attrs["stage"] != "Proposal" {
		t.Errorf("patch did not write attrs: %+v", patched.Attrs)
	}
	if _, still := patched.Attrs["customer"]; still {
		t.Errorf("attrs merged instead of replacing — a removed field came back: %+v", patched.Attrs)
	}
}

func TestIntegration_FollowUpCanBeScheduledAndClosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_activities`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	due := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	act, err := repo.CreateActivity(ctx, ActivityInput{
		Type:       "Call",
		Subject:    "Chase the quote",
		OccurredAt: time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC),
		DueAt:      &due,
		Status:     "Planned",
		Attrs:      map[string]any{"reference": "FU-1", "relatedTo": "Alpha Ltd"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if act.DueAt == nil || !act.DueAt.Equal(due) {
		t.Fatalf("due date dropped on create: %+v", act.DueAt)
	}
	if act.Status != "Planned" {
		t.Fatalf("status dropped on create: %q", act.Status)
	}

	// The case that could never happen before: marking a follow-up Done.
	done, err := repo.PatchActivity(ctx, act.ID, map[string]any{"status": "Done"})
	if err != nil {
		t.Fatalf("patch status: %v", err)
	}
	if done.Status != "Done" {
		t.Errorf("follow-up still reads %q after being marked Done", done.Status)
	}

	// occurred_at was create-only; correcting a mistyped date must stick.
	corrected, err := repo.PatchActivity(ctx, act.ID, map[string]any{
		"occurred_at": "2026-09-02T08:00:00Z",
		"due_at":      "2026-10-15",
	})
	if err != nil {
		t.Fatalf("patch dates: %v", err)
	}
	if corrected.OccurredAt.Day() != 2 {
		t.Errorf("occurred_at edit did not stick: %v", corrected.OccurredAt)
	}
	if corrected.DueAt == nil || corrected.DueAt.Month() != time.October {
		t.Errorf("due_at edit did not stick: %v", corrected.DueAt)
	}

	// Clearing a date the operator set by mistake.
	cleared, err := repo.PatchActivity(ctx, act.ID, map[string]any{"due_at": ""})
	if err != nil {
		t.Fatalf("clear due_at: %v", err)
	}
	if cleared.DueAt != nil {
		t.Errorf("due_at = %v after being cleared, want nil", cleared.DueAt)
	}
}

func TestIntegration_ComplaintRecordsItsOutcome(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_tickets`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	ticket, err := repo.CreateTicket(ctx, TicketInput{
		Subject:     "Damaged sack",
		Description: "Two sacks arrived torn",
		Priority:    "High",
		Status:      "Open",
		Attrs:       map[string]any{"reference": "CMP-1"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ticket.Attrs["reference"] != "CMP-1" {
		t.Errorf("ticket attrs dropped on create: %+v", ticket.Attrs)
	}

	resolved, err := repo.PatchTicket(ctx, ticket.ID, map[string]any{
		"status":      "Resolved",
		"resolution":  "Replaced both sacks and credited the freight",
		"resolved_at": "2026-09-05T14:30:00Z",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Resolution == "" {
		t.Error("a complaint was closed with no recorded outcome — resolution did not save")
	}
	if resolved.ResolvedAt == nil {
		t.Error("resolved_at did not save")
	}

	// Prove it against a fresh read, not the patch's own return value.
	fetched, err := repo.GetTicket(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Resolution != "Replaced both sacks and credited the freight" {
		t.Errorf("resolution read back as %q", fetched.Resolution)
	}
}

func TestIntegration_DealCloseDateIsEditable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_deals`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	deal, err := repo.CreateDeal(ctx, DealInput{
		Name:      "Q4 renewal",
		CloseDate: &first,
		Attrs:     map[string]any{"salesOrder": "SO-88"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if deal.Attrs["salesOrder"] != "SO-88" {
		t.Errorf("deal attrs dropped on create: %+v", deal.Attrs)
	}

	// This is the edit that used to return 200 and change nothing.
	moved, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"close_date": "2026-11-20"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if moved.CloseDate == nil || moved.CloseDate.Month() != time.November {
		t.Errorf("close_date = %v after the edit, want November — the edit was discarded", moved.CloseDate)
	}
}
