package models

import (
	"testing"
	"time"
)

// The six groups iag-authentication seeds must each land on a role, and only
// the rep role may be owner-scoped. Before this every crm-* group fell to the
// default and every platform user saw only rows carrying their own email.
func TestRoleFromGroupsMapsSeededCRMGroups(t *testing.T) {
	cases := map[string]string{
		"crm-administrator": "md",
		"crm-sales-manager": "head_commercial",
		"crm-marketing":     "head_marketing",
		"crm-sales-rep":     "sales_rep",
		"crm-support":       "support",
		"crm-viewer":        "viewer",
		"CRM Sales Manager": "head_commercial",
		"something-else":    "sales_rep",
	}
	for group, want := range cases {
		if got := RoleFromGroups([]string{group}, false); got != want {
			t.Errorf("RoleFromGroups(%q) = %q, want %q", group, got, want)
		}
	}
	if got := RoleFromGroups([]string{"crm-viewer"}, true); got != "md" {
		t.Errorf("superuser must win over groups, got %q", got)
	}
	for role := range Roles {
		if IsScopedRole(role) && role != "sales_rep" {
			t.Errorf("role %q is scoped; only sales_rep should be", role)
		}
	}
	for _, role := range []string{"support", "viewer"} {
		if _, ok := Roles[role]; !ok {
			t.Errorf("role %q has no RoleSpec; bootstrap would return no pages", role)
		}
	}
}

func TestVocabHelpers(t *testing.T) {
	if !InVocab("open", TicketStatuses) || InVocab("Open", TicketStatuses) {
		t.Error("InVocab must be exact-case")
	}
	if SLAFor("P1") != 4*time.Hour || SLAFor("nonsense") != SLAFor("P2") {
		t.Error("SLAFor: P1 is 4h and unknown falls back to P2")
	}
	for code, ok := range map[string]bool{"UGX": true, "usd": false, "US": false, "USDX": false, "": false} {
		if ValidCurrency(code) != ok {
			t.Errorf("ValidCurrency(%q) = %v", code, !ok)
		}
	}
	if !InRange(0, 0, 100) || !InRange(100, 0, 100) || InRange(101, 0, 100) || InRange(-1, 0, 100) {
		t.Error("InRange bounds are inclusive 0..100")
	}
}
