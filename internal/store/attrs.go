package store

import (
	"fmt"
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

// AttrsPatch is what a sparse PATCH body said about `attrs`, ready to be
// turned into one SET clause by SetExpr/Arg.
type AttrsPatch struct {
	Attrs   map[string]any
	Replace bool
}

// patchAttrs reads an `attrs` object out of a sparse PATCH body.
//
// Semantics are **merge, with null as delete**. The key absent → attrs are
// left alone. An object → its keys overwrite the stored ones, a key set to
// JSON null is removed, and every key the client did not mention survives.
// Explicit `attrs: null` → the whole map is cleared.
//
// It used to be replace. That matched the record form, which submits every
// field it owns — but it also wiped every key the form did NOT own: anything
// the bridge, a journey step or another client had put there vanished on the
// next app save, silently. Merge keeps those; null keeps "clear this field"
// possible, which is the one thing replace had going for it.
func patchAttrs(patch map[string]any) (AttrsPatch, bool) {
	raw, ok := patch["attrs"]
	if !ok {
		return AttrsPatch{}, false
	}
	switch v := raw.(type) {
	case map[string]any:
		return AttrsPatch{Attrs: v}, true
	case nil:
		return AttrsPatch{Attrs: map[string]any{}, Replace: true}, true
	default:
		return AttrsPatch{}, false
	}
}

// SetExpr is the SQL for the attrs column at placeholder $i.
func (p AttrsPatch) SetExpr(i int) string {
	if p.Replace {
		return fmt.Sprintf("attrs = $%d::jsonb", i)
	}
	return fmt.Sprintf("attrs = jsonb_strip_nulls(COALESCE(attrs, '{}'::jsonb) || $%d::jsonb)", i)
}

// Arg is the JSON bound to that placeholder. Nulls are kept on purpose:
// jsonb_strip_nulls on the merged result is what turns them into deletions.
func (p AttrsPatch) Arg() []byte {
	if len(p.Attrs) == 0 {
		return []byte("{}")
	}
	raw, err := json.Marshal(p.Attrs)
	if err != nil {
		return []byte("{}")
	}
	return raw
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
