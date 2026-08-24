package store

import (
	"testing"
	"time"
)

func TestDecodeAttrsNeverReturnsNil(t *testing.T) {
	for name, raw := range map[string][]byte{
		"nil":       nil,
		"empty":     {},
		"null":      []byte("null"),
		"malformed": []byte("{not json"),
		"array":     []byte(`["wrong","shape"]`),
	} {
		got := decodeAttrs(raw)
		if got == nil {
			t.Errorf("%s: decodeAttrs returned nil; callers range over it without a nil check", name)
		}
		if len(got) != 0 {
			t.Errorf("%s: expected empty map, got %v", name, got)
		}
	}
}

func TestDecodeAttrsRoundTrip(t *testing.T) {
	got := decodeAttrs([]byte(`{"stage":"Proposal","estimatedValue":"4200"}`))
	if got["stage"] != "Proposal" {
		t.Errorf("stage = %v", got["stage"])
	}
	if got["estimatedValue"] != "4200" {
		t.Errorf("estimatedValue = %v", got["estimatedValue"])
	}
}

// The column is NOT NULL, so encode must never produce a NULL or a bare literal
// that would fail the constraint.
func TestEncodeAttrsEmptyIsEmptyObject(t *testing.T) {
	for name, in := range map[string]map[string]any{
		"nil":   nil,
		"empty": {},
	} {
		if got := string(encodeAttrs(in)); got != "{}" {
			t.Errorf("%s: encodeAttrs = %q, want {}", name, got)
		}
	}
}

// Replace, not merge — otherwise a value the operator deleted comes back on the
// next read, because there is no way to express removal.
func TestPatchAttrsIsReplaceNotMerge(t *testing.T) {
	attrs, ok := patchAttrs(map[string]any{"attrs": map[string]any{"only": "this"}})
	if !ok {
		t.Fatal("attrs key present but not read")
	}
	if len(attrs) != 1 || attrs["only"] != "this" {
		t.Errorf("attrs = %v, want exactly {only: this}", attrs)
	}

	if cleared, ok := patchAttrs(map[string]any{"attrs": nil}); !ok || len(cleared) != 0 {
		t.Errorf("explicit null should clear attrs, got %v ok=%v", cleared, ok)
	}
}

// An absent key must leave the column alone: a sparse PATCH that touches only
// `status` must not wipe every overflow field the record holds.
func TestPatchAttrsAbsentLeavesColumnAlone(t *testing.T) {
	if _, ok := patchAttrs(map[string]any{"status": "Done"}); ok {
		t.Error("absent attrs key reported as present; a sparse patch would erase the column")
	}
	if _, ok := patchAttrs(map[string]any{"attrs": "not an object"}); ok {
		t.Error("wrong-typed attrs should be ignored, not applied")
	}
}

func TestParsePatchTime(t *testing.T) {
	want := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"2026-08-24", "2026-08-24T00:00:00Z", "2026-08-24T00:00:00"} {
		got, ok := parsePatchTime(in).(time.Time)
		if !ok || !got.Equal(want) {
			t.Errorf("parsePatchTime(%q) = %v, want %v", in, got, want)
		}
	}
	// Clearing a date an operator set by mistake has to be possible.
	for _, in := range []any{"", nil, 42, "gibberish"} {
		if got := parsePatchTime(in); got != nil {
			t.Errorf("parsePatchTime(%v) = %v, want nil", in, got)
		}
	}
}
