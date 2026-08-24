package handlers

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// The Next.js CRM store submits form values as strings (e.g.
// {"amount":"5000","probability":"60","dms_linked":"true"}), but the *Input
// structs declare those fields as float64/int/bool. Plain ShouldBindJSON would
// then 400 with `json: cannot unmarshal string into Go struct field`. These
// helpers re-encode string-encoded scalars to the destination field's kind
// before the final decode. Nested struct and slice-of-struct fields (e.g.
// QuoteInput.LineItems) are recursed into. String-typed fields
// (AccountInput.Value, EngineerInput.Active) are never touched because
// coercion only fires when the target kind is numeric/bool.

// bindJSONCoerced decodes a single JSON object body into dst, first coercing
// string-encoded numeric/bool fields to dst's scalar field kinds.
func bindJSONCoerced(c *gin.Context, dst any) error {
	raw, err := c.GetRawData()
	if err != nil {
		return err
	}
	return json.Unmarshal(coerceJSONScalars(raw, dst), dst)
}

func coerceScalarStrings(t reflect.Type, m map[string]any) {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		val, present := m[name]
		if !present {
			continue
		}
		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if s, ok := val.(string); ok && s == "" {
			switch ft.Kind() {
			case reflect.Float32, reflect.Float64,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Bool:
				// Blank numeric/bool form value clears the field: JSON null decodes
				// to zero (value) or nil (pointer); "" would fail the unmarshal.
				m[name] = nil
				continue
			}
		}
		// Dates are the same problem as numbers, and worse: a date input yields
		// either "" (nothing picked) or "2026-08-24" (date only), and BOTH fail
		// to unmarshal into time.Time — 400ing the entire request over one
		// optional field, so a whole complaint is refused because nobody typed a
		// resolved date. Blank clears; date-only widens to midnight UTC.
		if ft == timeType {
			if s, ok := val.(string); ok {
				m[name] = coerceTimeString(s)
				continue
			}
		}
		switch ft.Kind() {
		case reflect.Struct:
			if nested, ok := val.(map[string]any); ok {
				coerceScalarStrings(ft, nested)
			}
		case reflect.Slice, reflect.Array:
			et := ft.Elem()
			for et.Kind() == reflect.Ptr {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				if arr, ok := val.([]any); ok {
					for _, el := range arr {
						if nested, ok := el.(map[string]any); ok {
							coerceScalarStrings(et, nested)
						}
					}
				}
			}
		case reflect.Float32, reflect.Float64:
			if s, ok := val.(string); ok && s != "" {
				if v, err := strconv.ParseFloat(s, 64); err == nil {
					m[name] = v
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if s, ok := val.(string); ok && s != "" {
				if v, err := strconv.ParseInt(s, 10, 64); err == nil {
					m[name] = v
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if s, ok := val.(string); ok && s != "" {
				if v, err := strconv.ParseUint(s, 10, 64); err == nil {
					m[name] = v
				}
			}
		case reflect.Bool:
			if s, ok := val.(string); ok && s != "" {
				if v, err := strconv.ParseBool(s); err == nil {
					m[name] = v
				}
			}
		}
	}
}

func coerceJSONScalars(raw []byte, dst any) []byte {
	t := reflect.TypeOf(dst)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return raw
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		var rows []map[string]any
		if json.Unmarshal(raw, &rows) != nil {
			return raw
		}
		et := t.Elem()
		for _, m := range rows {
			coerceScalarStrings(et, m)
		}
		if out, err := json.Marshal(rows); err == nil {
			return out
		}
	case reflect.Struct:
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			return raw
		}
		coerceScalarStrings(t, m)
		if out, err := json.Marshal(m); err == nil {
			return out
		}
	}
	return raw
}

var timeType = reflect.TypeOf(time.Time{})

// coerceTimeString normalises a form date onto something time.Time will accept,
// or nil to leave the field unset. An unparseable value becomes nil rather than
// an error, matching how this file treats every other wrong-shaped scalar: the
// field is dropped, the request still lands.
func coerceTimeString(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return nil
}
