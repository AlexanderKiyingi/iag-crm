package store

import "context"

// SchemaStatus reports what the database has actually had applied to it, as
// opposed to what the running binary expects.
//
// This exists because the gap between those two is invisible from the outside
// and expensive to diagnose. A deploy that ships a migration together with the
// code reading its columns leaves the service running and answering health
// checks — `/ready` says database: true, because the connection is fine — while
// every query naming a new column fails with a generic 500. Nothing in the
// service says "the schema is behind"; you have to infer it from which
// endpoints break and which do not.
type SchemaStatus struct {
	// AppliedMigrations is every version recorded in crm.schema_migrations.
	AppliedMigrations []string `json:"applied_migrations"`
	// LatestMigration is the highest version applied, or "" on a bare database.
	LatestMigration string `json:"latest_migration"`
}

// SchemaStatus reads the migration ledger. Errors are returned rather than
// swallowed so a caller can tell "no migrations applied" from "could not look".
func (r *Repository) SchemaStatus(ctx context.Context) (SchemaStatus, error) {
	var out SchemaStatus
	rows, err := r.db(ctx).Query(ctx, `
		SELECT version FROM crm.schema_migrations ORDER BY version
	`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return out, err
		}
		out.AppliedMigrations = append(out.AppliedMigrations, v)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if n := len(out.AppliedMigrations); n > 0 {
		out.LatestMigration = out.AppliedMigrations[n-1]
	}
	return out, nil
}
