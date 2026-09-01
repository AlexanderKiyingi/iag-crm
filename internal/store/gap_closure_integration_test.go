package store

import (
	"context"
	"testing"
	"time"
)

// The gaps these cover were all found by round-tripping the live client, not by
// reading this package. Each one returned 200 and wrote nothing, or wrote a
// value the client never sent — the failure mode a struct-tag review cannot
// see, so every assertion here is a read-back off a fresh query.

func TestIntegration_ComplaintKeepsTheDateItHappened(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_tickets`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	// A complaint logged today about last Friday's delivery. Before 0012 the
	// only date a ticket had was created_at, so this read as today's incident.
	happened := time.Date(2026, 3, 14, 9, 30, 0, 0, time.UTC)
	ticket, err := repo.CreateTicket(ctx, TicketInput{
		Subject:    "Short delivery",
		Account:    "Alpha Ltd",
		OccurredAt: &happened,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	fetched, err := repo.GetTicket(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.OccurredAt == nil || !fetched.OccurredAt.Equal(happened) {
		t.Fatalf("occurred_at not stored: got %v want %v", fetched.OccurredAt, happened)
	}
	if fetched.CreatedAt.Equal(happened) {
		t.Fatal("created_at was overwritten with the incident date; it is the audit trail, not the fact")
	}

	// Correcting the date has to work too — it is the field most likely to be
	// typed wrong, being the one nobody can read off the screen in front of them.
	corrected := time.Date(2026, 3, 13, 16, 0, 0, 0, time.UTC)
	if _, err := repo.PatchTicket(ctx, ticket.ID, map[string]any{
		"occurred_at": corrected.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	again, err := repo.GetTicket(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("get after patch: %v", err)
	}
	if again.OccurredAt == nil || !again.OccurredAt.Equal(corrected) {
		t.Fatalf("occurred_at not patched: got %v want %v", again.OccurredAt, corrected)
	}

	// Omitted on create it falls back to now, not NULL: every row that existed
	// before 0012 was backfilled from created_at, and a row written after it
	// should not be the only undated one.
	undated, err := repo.CreateTicket(ctx, TicketInput{Subject: "No date given"})
	if err != nil {
		t.Fatalf("create undated: %v", err)
	}
	if undated.OccurredAt == nil {
		t.Fatal("occurred_at left NULL when the client sent none")
	}
}

func TestIntegration_ComplaintCanBeMovedToAnotherCustomer(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, table := range []string{"crm_tickets", "crm_activities", "crm_accounts"} {
		if _, err := pool.Exec(ctx, `DELETE FROM `+table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	repo := New(pool)

	alpha, err := repo.CreateAccount(ctx, AccountInput{Name: "Alpha Ltd"})
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	beta, err := repo.CreateAccount(ctx, AccountInput{Name: "Beta Ltd"})
	if err != nil {
		t.Fatalf("create beta: %v", err)
	}

	// `account` was absent from PatchTicket's allowlist entirely, so filing a
	// complaint against the wrong customer was unrecoverable through the API:
	// the edit returned 200 and changed nothing.
	ticket, err := repo.CreateTicket(ctx, TicketInput{Subject: "Wrong customer", Account: "Alpha Ltd"})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if ticket.AccountID != alpha.ID {
		t.Fatalf("create did not link the account: got %q want %q", ticket.AccountID, alpha.ID)
	}
	moved, err := repo.PatchTicket(ctx, ticket.ID, map[string]any{"account": "Beta Ltd"})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if moved.Account != "Beta Ltd" {
		t.Fatalf("account name not moved: %q", moved.Account)
	}
	// The name alone is the trap: it is what the screen renders, while every
	// rollup and the account 360 join on account_id.
	if moved.AccountID != beta.ID {
		t.Fatalf("account_id left on the old account: got %q want %q", moved.AccountID, beta.ID)
	}

	// Same defect, same fix, on follow-ups.
	activity, err := repo.CreateActivity(ctx, ActivityInput{
		Subject: "Site visit", Type: "Visit", Account: "Alpha Ltd", OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	movedActivity, err := repo.PatchActivity(ctx, activity.ID, map[string]any{"account": "Beta Ltd"})
	if err != nil {
		t.Fatalf("patch activity: %v", err)
	}
	if movedActivity.Account != "Beta Ltd" || movedActivity.AccountID != beta.ID {
		t.Fatalf("activity account not re-pointed: name=%q id=%q want id=%q",
			movedActivity.Account, movedActivity.AccountID, beta.ID)
	}
}

func TestIntegration_LeadCarriesValueFollowUpAndAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, table := range []string{"crm_leads", "crm_accounts"} {
		if _, err := pool.Exec(ctx, `DELETE FROM `+table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	repo := New(pool)

	alpha, err := repo.CreateAccount(ctx, AccountInput{Name: "Alpha Ltd"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	due := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	value := 4200.0
	lead, err := repo.CreateLead(ctx, LeadInput{
		Name:           "Promoted Lead",
		Email:          "promoted@example.com",
		Account:        "Alpha Ltd",
		NextFollowUpAt: &due,
		EstimatedValue: &value,
		Currency:       "UGX",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	fetched, err := repo.GetLead(ctx, lead.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.AccountID != alpha.ID || fetched.Account != "Alpha Ltd" {
		t.Fatalf("lead not linked to its account: id=%q name=%q", fetched.AccountID, fetched.Account)
	}
	if fetched.NextFollowUpAt == nil || !fetched.NextFollowUpAt.Equal(due) {
		t.Fatalf("next_follow_up_at not stored: %v", fetched.NextFollowUpAt)
	}
	if fetched.EstimatedValue == nil || *fetched.EstimatedValue != value {
		t.Fatalf("estimated_value not stored: %v", fetched.EstimatedValue)
	}
	if fetched.Currency != "UGX" {
		t.Fatalf("currency not stored: %q", fetched.Currency)
	}

	// These are columns now precisely so a query can reach them — the point of
	// promoting them out of attrs. Asserted in SQL rather than through the
	// repository, because reading them back through Go would pass just as well
	// if they were still JSONB.
	var reachable int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM crm_leads
		WHERE next_follow_up_at IS NOT NULL AND estimated_value > 1000
	`).Scan(&reachable); err != nil {
		t.Fatalf("query: %v", err)
	}
	if reachable != 1 {
		t.Fatalf("lead is not reachable by a query on the promoted columns: got %d", reachable)
	}

	// A value an operator deletes must actually go. Clearing is why
	// parsePatchNumber and parsePatchTime treat "" as NULL rather than ignoring
	// it: an ignored key is a field that can be set once and never retracted.
	if _, err := repo.PatchLead(ctx, lead.ID, map[string]any{
		"estimated_value":   "",
		"next_follow_up_at": "",
	}); err != nil {
		t.Fatalf("patch clear: %v", err)
	}
	cleared, err := repo.GetLead(ctx, lead.ID)
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if cleared.EstimatedValue != nil || cleared.NextFollowUpAt != nil {
		t.Fatalf("cleared values came back: value=%v followUp=%v", cleared.EstimatedValue, cleared.NextFollowUpAt)
	}

	// And re-pointing the customer must move the FK, not just the label.
	beta, err := repo.CreateAccount(ctx, AccountInput{Name: "Beta Ltd"})
	if err != nil {
		t.Fatalf("create beta: %v", err)
	}
	moved, err := repo.PatchLead(ctx, lead.ID, map[string]any{"account": "Beta Ltd"})
	if err != nil {
		t.Fatalf("patch account: %v", err)
	}
	if moved.AccountID != beta.ID {
		t.Fatalf("lead account_id not re-pointed: got %q want %q", moved.AccountID, beta.ID)
	}
}

func TestIntegration_PlainPatchStampsTheCloseDate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM crm_deals`); err != nil {
		t.Fatalf("clean: %v", err)
	}
	repo := New(pool)

	deal, err := repo.CreateDeal(ctx, DealInput{Name: "Won by PATCH", Amount: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// The clients move a deal with a plain PATCH — it already books the finance
	// AR reference through finalizeDealWon, so nothing pushed anyone toward
	// /deals/:id/stage, and won_at was written only there. PipelineSummary reads
	// AVG(COALESCE(won_at, NOW()) - created_at), so a NULL here does not read as
	// missing: it reads as "still open", for ever.
	if _, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"stage": "won"}); err != nil {
		t.Fatalf("patch won: %v", err)
	}
	var wonAt, lostAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT won_at, lost_at FROM crm_deals WHERE id = $1`, deal.ID).Scan(&wonAt, &lostAt); err != nil {
		t.Fatalf("read won_at: %v", err)
	}
	if wonAt == nil {
		t.Fatal("won_at still NULL after a plain PATCH to stage=won")
	}
	if lostAt != nil {
		t.Fatal("lost_at set on a won deal")
	}
	first := *wonAt

	// An unrelated edit must not move the date a deal closed.
	if _, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"stage": "won", "description": "corrected"}); err != nil {
		t.Fatalf("patch again: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT won_at FROM crm_deals WHERE id = $1`, deal.ID).Scan(&wonAt); err != nil {
		t.Fatalf("read won_at again: %v", err)
	}
	if wonAt == nil || !wonAt.Equal(first) {
		t.Fatalf("won_at moved on an unrelated edit: %v -> %v", first, wonAt)
	}

	// Reopening clears it. A deal that is no longer won has no close date, and
	// leaving one behind is what makes win-rate and velocity disagree with the
	// stage column they are computed beside.
	if _, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"stage": "negotiation"}); err != nil {
		t.Fatalf("patch reopen: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT won_at FROM crm_deals WHERE id = $1`, deal.ID).Scan(&wonAt); err != nil {
		t.Fatalf("read won_at after reopen: %v", err)
	}
	if wonAt != nil {
		t.Fatalf("won_at survived a reopen: %v", wonAt)
	}

	// Lost is the same rule on the other column.
	if _, err := repo.PatchDeal(ctx, deal.ID, map[string]any{"stage": "lost"}); err != nil {
		t.Fatalf("patch lost: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT won_at, lost_at FROM crm_deals WHERE id = $1`, deal.ID).Scan(&wonAt, &lostAt); err != nil {
		t.Fatalf("read after lost: %v", err)
	}
	if lostAt == nil || wonAt != nil {
		t.Fatalf("lost deal has won_at=%v lost_at=%v", wonAt, lostAt)
	}
}
