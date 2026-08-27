package store

import (
	"context"
	"regexp"
	"strings"
)

// LegacyCodeSource is the source_service recorded in crm_external_refs for the
// prefixed identifiers CRM minted before its keys became uuid (migration 0011).
const LegacyCodeSource = "crm-legacy-code"

var uuidShape = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ResolveLegacyID maps one of the old prefixed identifiers (ACC-500, DEAL-500)
// onto the uuid it became, so a bookmark or a printed document that quotes the
// old code still finds its row.
//
// Anything already shaped like a uuid is returned untouched without a query.
// An unknown value is also returned untouched — it may belong to another
// service, and the caller's own lookup should decide whether it exists rather
// than this helper turning it into a different error.
//
// A code is only rewritten when it maps to exactly one row. The uniqueness
// constraint on crm_external_refs is per (source_service, source_type,
// source_id), so in principle two tables could have minted the same code; the
// prefixes make that near-impossible in practice, but an ambiguous code is left
// alone rather than resolved to an arbitrary one of them.
func (r *Repository) ResolveLegacyID(ctx context.Context, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || uuidShape.MatchString(id) {
		return id
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT target_id FROM crm_external_refs
		 WHERE source_service = $1 AND source_id = $2
		 LIMIT 2
	`, LegacyCodeSource, id)
	if err != nil {
		return id
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			return id
		}
		found = append(found, target)
	}
	if rows.Err() != nil || len(found) != 1 {
		return id
	}
	return found[0]
}
