package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

type coerceDateFixture struct {
	Subject    string     `json:"subject"`
	Amount     float64    `json:"amount"`
	OccurredAt time.Time  `json:"occurred_at"`
	DueAt      *time.Time `json:"due_at"`
}

func decodeFixture(t *testing.T, body string) coerceDateFixture {
	t.Helper()
	var out coerceDateFixture
	coerced := coerceJSONScalars([]byte(body), &out)
	if err := json.Unmarshal(coerced, &out); err != nil {
		t.Fatalf("unmarshal after coercion: %v (payload %s)", err, coerced)
	}
	return out
}

// A date input that nobody filled in submits "", which fails to unmarshal into
// time.Time and 400s the whole request — refusing a complaint because its
// optional resolved-date is blank.
func TestCoerceBlankDateDoesNotFailTheRequest(t *testing.T) {
	got := decodeFixture(t, `{"subject":"Late delivery","occurred_at":"","due_at":""}`)
	if !got.OccurredAt.IsZero() {
		t.Errorf("blank occurred_at should stay zero, got %v", got.OccurredAt)
	}
	if got.DueAt != nil {
		t.Errorf("blank due_at should stay nil, got %v", got.DueAt)
	}
	if got.Subject != "Late delivery" {
		t.Errorf("subject lost: %q", got.Subject)
	}
}

// An <input type="date"> submits "2026-08-24" with no time component, which
// time.Time also rejects.
func TestCoerceDateOnlyWidensToMidnightUTC(t *testing.T) {
	got := decodeFixture(t, `{"occurred_at":"2026-08-24","due_at":"2026-08-31"}`)
	want := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if !got.OccurredAt.Equal(want) {
		t.Errorf("occurred_at = %v, want %v", got.OccurredAt, want)
	}
	if got.DueAt == nil {
		t.Fatal("due_at should be set")
	}
	if wantDue := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC); !got.DueAt.Equal(wantDue) {
		t.Errorf("due_at = %v, want %v", got.DueAt, wantDue)
	}
}

func TestCoerceKeepsFullTimestamps(t *testing.T) {
	got := decodeFixture(t, `{"occurred_at":"2026-08-24T09:30:00Z"}`)
	want := time.Date(2026, 8, 24, 9, 30, 0, 0, time.UTC)
	if !got.OccurredAt.Equal(want) {
		t.Errorf("occurred_at = %v, want %v", got.OccurredAt, want)
	}
}

// Nonsense in a date field drops that field rather than rejecting the record —
// the same contract every other wrong-shaped scalar gets in this file.
func TestCoerceGarbageDateIsDroppedNotFatal(t *testing.T) {
	got := decodeFixture(t, `{"subject":"kept","occurred_at":"not a date"}`)
	if !got.OccurredAt.IsZero() {
		t.Errorf("garbage date should clear the field, got %v", got.OccurredAt)
	}
	if got.Subject != "kept" {
		t.Errorf("subject lost: %q", got.Subject)
	}
}

// The behaviour this file was written for must survive the date handling.
func TestCoerceStillHandlesStringEncodedNumbers(t *testing.T) {
	got := decodeFixture(t, `{"amount":"5000"}`)
	if got.Amount != 5000 {
		t.Errorf("amount = %v, want 5000", got.Amount)
	}
}
