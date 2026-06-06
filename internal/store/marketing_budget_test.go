package store

import "testing"

func TestBudgetBurnPct(t *testing.T) {
	if got := budgetBurnPct(36000, 18000); got != 50 {
		t.Fatalf("got %v want 50", got)
	}
	if got := budgetBurnPct(0, 1000); got != 0 {
		t.Fatalf("got %v want 0", got)
	}
	if got := budgetBurnPct(100, 250); got != 100 {
		t.Fatalf("got %v want capped 100", got)
	}
}
