package store

import (
	"os"
	"regexp"
	"testing"
)

// Every list and single-record read of activities, tickets and quotes must
// run the link-name fill, or the record clients — which hold one string per
// field — read back a blank contact/deal on the list and a name on the single
// fetch. A source-level pin, because the fill is a plain call that a rewrite
// of a function's tail silently loses (it happened in the scope change).
func TestLinkNameFillIsCalledFromEveryReadPath(t *testing.T) {
	src, err := os.ReadFile("resources.go")
	if err != nil {
		t.Fatal(err)
	}
	platform, err := os.ReadFile("platform.go")
	if err != nil {
		t.Fatal(err)
	}
	all := string(src) + string(platform)
	funcs := map[string]string{
		"ListActivities": "fillActivityLinks",
		"GetActivity":    "fillActivityLinks",
		"ListTickets":    "fillTicketLinks",
		"ListQuotes":     "fillQuoteLinks",
	}
	for fn, fill := range funcs {
		body := functionBody(all, fn)
		if body == "" {
			t.Errorf("%s not found", fn)
			continue
		}
		if !regexp.MustCompile(fill + `\(`).MatchString(body) {
			t.Errorf("%s does not call %s — list and single reads will disagree on contact/deal names", fn, fill)
		}
	}
	for _, fn := range []string{"GetTicket", "CreateTicket", "PatchTicket", "GetQuote", "CreateQuote", "PatchQuote"} {
		body := functionBody(all, fn)
		// Either fills itself or delegates to the Get*, which does.
		if body != "" && !regexp.MustCompile(`WithLinks\(|return r\.Get(Ticket|Quote)\(`).MatchString(body) {
			t.Errorf("%s returns a scanned row without the link fill", fn)
		}
	}
}

// functionBody returns the text of `func (r *Repository) name(` up to the
// next top-level func, or "" when absent.
func functionBody(src, name string) string {
	re := regexp.MustCompile(`(?s)func \(r \*Repository\) ` + name + `\(.*?\n}\n`)
	m := re.FindString(src)
	return m
}
