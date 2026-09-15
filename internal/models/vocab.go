package models

import (
	"strings"
	"time"
)

// Controlled vocabularies for the columns a query, a report or a workflow
// reads. Deal stage was the first: the clients rendered their own words, wrote
// them raw, and `WHERE stage IN ('negotiation','proposal')` silently returned
// nothing. Lead status went the same way (0012). These are the rest, declared
// once so a handler can refuse a value no query will ever match instead of
// storing it.
//
// Every list is lowercase snake_case. Clients translate their display words on
// the way in and out — never the service.

const (
	TicketStatusOpen       = "open"
	TicketStatusInProgress = "in_progress"
	TicketStatusResolved   = "resolved"
	TicketStatusClosed     = "closed"

	// Priorities keep the P-scale the service has always defaulted to (P2);
	// the SLA table below is keyed on them.
	TicketPriorityP1 = "P1"
	TicketPriorityP2 = "P2"
	TicketPriorityP3 = "P3"
	TicketPriorityP4 = "P4"

	ActivityStatusPlanned   = "planned"
	ActivityStatusDone      = "done"
	ActivityStatusCancelled = "cancelled"

	ContactStatusActive   = "active"
	ContactStatusInactive = "inactive"

	LeadStatusQualifying = "qualifying"
	LeadStatusConverted  = "converted"
	LeadStatusClosed     = "closed"
)

var (
	TicketStatuses    = []string{TicketStatusOpen, TicketStatusInProgress, TicketStatusResolved, TicketStatusClosed}
	TicketPriorities  = []string{TicketPriorityP1, TicketPriorityP2, TicketPriorityP3, TicketPriorityP4}
	ActivityStatuses  = []string{ActivityStatusPlanned, ActivityStatusDone, ActivityStatusCancelled}
	ContactStatuses   = []string{ContactStatusActive, ContactStatusInactive}
	LeadStatuses      = []string{LeadStatusQualifying, LeadStatusConverted, LeadStatusClosed}
	TicketChannels    = []string{"email", "phone", "whatsapp", "web", "visit", "other"}
)

// TicketSLA is how long a ticket of each priority has before it breaches.
// Before this every ticket got a flat 24h regardless of priority, so P1 and P4
// were due at the same moment and the figure meant nothing.
var TicketSLA = map[string]time.Duration{
	TicketPriorityP1: 4 * time.Hour,
	TicketPriorityP2: 24 * time.Hour,
	TicketPriorityP3: 72 * time.Hour,
	TicketPriorityP4: 7 * 24 * time.Hour,
}

// SLAFor returns the SLA window for a priority, falling back to P2's when the
// priority is unknown — the same default CreateTicket applies to the column.
func SLAFor(priority string) time.Duration {
	if d, ok := TicketSLA[priority]; ok {
		return d
	}
	return TicketSLA[TicketPriorityP2]
}

// InVocab reports whether value is one of allowed. The comparison is exact:
// a client that sends "Open" for "open" has not translated, and storing it
// would be the bug this file exists to end.
func InVocab(value string, allowed []string) bool {
	for _, v := range allowed {
		if v == value {
			return true
		}
	}
	return false
}

// VocabError names the field and the accepted values so a 400 is actionable.
func VocabError(field string, allowed []string) string {
	return field + " must be one of: " + strings.Join(allowed, ", ")
}

// InRange is the 0–100 check shared by lead score and deal probability. Both
// columns are ints with no constraint, and BumpLeadScore caps at 100 on the
// way up but nothing stopped a client writing 250 or -5 directly.
func InRange(v, lo, hi int) bool {
	return v >= lo && v <= hi
}

// ValidCurrency accepts a three-letter uppercase code. Not a full ISO-4217
// table — the overview sums amounts by currency, and the failure being guarded
// against is "UGX" beside "ugx" beside "Ugandan shillings", not a made-up code.
func ValidCurrency(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// DefaultCurrency is applied when a client sends none. UGX because that is what
// the overview series and the finance AR booking assume when unlabelled.
const DefaultCurrency = "UGX"
