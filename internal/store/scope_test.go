package store

import (
	"strings"
	"testing"
)

// applyScope is the enforcement point for record visibility, so its behaviour is
// worth pinning precisely. The bug it exists to prevent: scoping used to work by
// defaulting ListOpts.Owner to the caller's email when ?owner= was absent, which
// meant passing ?owner=someone@else skipped the scope entirely and returned
// another rep's records. Keeping the enforced scope in its own field makes the
// two predicates AND together instead of one replacing the other.

func TestApplyScope_addsPredicateWhenScoped(t *testing.T) {
	where := []string{"1=1"}
	args := []any{}
	i := 1

	where, args = applyScope(ListOpts{ScopeOwner: "rep@iag.com"}, "owner", where, args, &i)

	if len(where) != 2 || !strings.Contains(where[1], "owner = $1") {
		t.Fatalf("where = %v, want an owner predicate", where)
	}
	if len(args) != 1 || args[0] != "rep@iag.com" {
		t.Fatalf("args = %v, want the scope owner bound", args)
	}
	if i != 2 {
		t.Fatalf("placeholder index = %d, want it advanced to 2", i)
	}
}

func TestApplyScope_noopWhenUnscoped(t *testing.T) {
	where := []string{"1=1"}
	args := []any{}
	i := 1

	where, args = applyScope(ListOpts{}, "owner", where, args, &i)

	if len(where) != 1 || len(args) != 0 || i != 1 {
		t.Fatalf("unscoped caller must be unrestricted, got where=%v args=%v i=%d", where, args, i)
	}
}

// The regression itself: a scoped rep supplying ?owner= must end up with BOTH
// predicates, not have their scope replaced by the value they supplied.
func TestApplyScope_userFilterCannotWidenScope(t *testing.T) {
	opts := ListOpts{
		ScopeOwner: "rep@iag.com",      // enforced, from identity
		Owner:      "director@iag.com", // supplied, from ?owner=
	}

	where := []string{"1=1"}
	args := []any{}
	i := 1

	// Enforced scope first, exactly as the list queries do.
	where, args = applyScope(opts, "owner", where, args, &i)
	// Then the caller's own filter, as the query builder appends it.
	if opts.Owner != "" {
		where = append(where, "owner = $2")
		args = append(args, opts.Owner)
		i++
	}

	sql := strings.Join(where, " AND ")
	if !strings.Contains(sql, "owner = $1") || !strings.Contains(sql, "owner = $2") {
		t.Fatalf("both predicates must survive; got %q", sql)
	}
	if args[0] != "rep@iag.com" {
		t.Fatalf("the enforced scope must be bound first, got %v", args)
	}
	// The two conflicting values AND to an empty result, which is the correct
	// outcome: a rep asking for someone else's records gets none, rather than
	// getting them.
	if args[1] != "director@iag.com" {
		t.Fatalf("the caller's filter should still be applied, got %v", args)
	}
}

// A scope value must be bound as a parameter, never interpolated — the owner
// comes from a JWT claim, and a claim is still input.
func TestApplyScope_bindsRatherThanInterpolates(t *testing.T) {
	where := []string{"1=1"}
	args := []any{}
	i := 1

	evil := "x' OR '1'='1"
	where, _ = applyScope(ListOpts{ScopeOwner: evil}, "owner", where, args, &i)

	if strings.Contains(strings.Join(where, " AND "), evil) {
		t.Fatal("scope value was interpolated into SQL; it must be a bound parameter")
	}
}
