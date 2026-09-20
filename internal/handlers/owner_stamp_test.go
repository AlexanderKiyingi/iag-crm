package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every table read through scopedListOpts is owner-scoped: a sales rep sees
// only rows carrying their own email. Five of the eight create handlers
// stamped the caller onto a blank owner; accounts, quotes and campaigns did
// not, so a rep who left Owner blank — the app's form says to — created a
// row they could not see. The list below is the set of owner-scoped tables
// with a create verb; a handler added without the stamp fails here rather
// than in a rep's empty list.
func TestEveryScopedCreateStampsOwner(t *testing.T) {
	src, err := os.ReadFile("entities.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"CreateAccount", "CreateContact", "CreateLead", "CreateDeal",
		"CreateActivity", "CreateTicket", "CreateQuote", "CreateCampaign",
	} {
		body := handlerBody(string(src), name)
		if body == "" {
			t.Errorf("%s: handler not found in entities.go", name)
			continue
		}
		if !strings.Contains(body, "stampOwner(c, &in.Owner)") {
			t.Errorf("%s does not stamp the caller onto a blank owner", name)
		}
	}
}

// handlerBody returns the text of `func (h *API) <name>(` up to the next
// top-level func, or "" when it is absent.
func handlerBody(src, name string) string {
	start := regexp.MustCompile(`(?m)^func \(h \*API\) ` + name + `\(`).FindStringIndex(src)
	if start == nil {
		return ""
	}
	rest := src[start[1]:]
	if next := regexp.MustCompile(`(?m)^func `).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	return rest
}
