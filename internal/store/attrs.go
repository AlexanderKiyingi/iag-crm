package store

import (
	"encoding/json"
	"strconv"
	"time"
)

// Free-form overflow for client fields this service has no promoted column for
// — see db/migrations/0008_entity_attrs.sql for what belongs here and what does
// not.

// decodeAttrs turns a JSONB column into the map the models expose. A NULL or
// unparseable column yields an empty map rather than nil, so callers never have
// to nil-check before ranging.
func decodeAttrs(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// encodeAttrs marshals attrs for storage, falling back to an empty object so a
// marshal failure can never write NULL into a NOT NULL column.
func encodeAttrs(attrs map[string]any) []byte {
	if len(attrs) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(attrs)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

// patchAttrs reads an `attrs` object out of a sparse PATCH body.
//
// Semantics are **replace, not merge**: the key is absent → attrs are left
// alone; the key is present → it becomes the whole map. That matches how the
// clients actually behave (the record form submits every field it owns on every
// edit) and it is the only version where clearing a field works — a merge would
// make removal impossible, so a value an operator deleted would come back on
// the next read.
func patchAttrs(patch map[string]any) (map[string]any, bool) {
	raw, ok := patch["attrs"]
	if !ok {
		return nil, false
	}
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case nil:
		return map[string]any{}, true
	default:
		return nil, false
	}
}

// parsePatchTime coerces a JSON value from a sparse PATCH body into a nullable
// timestamp. An empty string or null clears the column — that is the only way a
// client can retract a date it set by mistake. An unparseable value also clears
// rather than erroring, matching how the rest of Patch* treats a wrong-typed
// key: it is ignored, not fatal.
func parsePatchTime(v any) any {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return nil
}

// parsePatchNumber coerces a JSON value from a sparse PATCH body into a
// nullable numeric column.
//
// Null and the empty string clear the column — the same retraction rule
// parsePatchTime follows, and the only way an operator can undo a figure they
// entered by mistake. A string is accepted as well as a number because the
// record clients send every field as a string; refusing one would mean a value
// the operator can see in the form and cannot save.
func parsePatchNumber(v any) any {
	switch n := v.(type) {
	case nil:
		return nil
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		if n == "" {
			return nil
		}
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return nil
		}
		return f
	default:
		return nil
	}
}
