package store

import (
	"context"
	"testing"
)

// A value that is already a uuid, or is empty, must be returned without
// touching the database. The guard matters beyond saving a query: it is what
// lets every :id route run this on a request whose id was never a CRM code —
// /finance/invoices/:id and /procurement/purchase-orders/:id both do.
func TestResolveLegacyID_shortCircuitsWithoutDB(t *testing.T) {
	// A zero Repository has no pool, so any of these reaching a query panics.
	repo := &Repository{}
	ctx := context.Background()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"blank", "   ", ""},
		{"lowercase uuid", "338ab5cc-8fa5-3198-8071-9a90882d3bd5", "338ab5cc-8fa5-3198-8071-9a90882d3bd5"},
		{"uppercase uuid", "338AB5CC-8FA5-3198-8071-9A90882D3BD5", "338AB5CC-8FA5-3198-8071-9A90882D3BD5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := repo.ResolveLegacyID(ctx, tc.in); got != tc.want {
				t.Fatalf("ResolveLegacyID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestUUIDShape(t *testing.T) {
	valid := []string{
		"338ab5cc-8fa5-3198-8071-9a90882d3bd5",
		"00000000-0000-0000-0000-000000000000",
	}
	for _, v := range valid {
		if !uuidShape.MatchString(v) {
			t.Errorf("expected %q to read as a uuid", v)
		}
	}
	// Every one of these must fall through to a legacy lookup rather than being
	// mistaken for a uuid.
	invalid := []string{
		"ACC-500", "DEAL-500", "CON-1200", "LEAD-200",
		"338ab5cc8fa531988071 9a90882d3bd5",
		"338ab5cc-8fa5-3198-8071-9a90882d3bd",   // a digit short
		"338ab5cc-8fa5-3198-8071-9a90882d3bd55", // a digit over
		"zzzzzzzz-8fa5-3198-8071-9a90882d3bd5",
	}
	for _, v := range invalid {
		if uuidShape.MatchString(v) {
			t.Errorf("expected %q NOT to read as a uuid", v)
		}
	}
}
