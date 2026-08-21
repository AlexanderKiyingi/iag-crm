package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ModuleRecord is a generic CRM-owned record for lightweight modules that have
// no dedicated table (products, services, solutions, projects, documents, …).
// Display columns are stored flat for listing/sorting; the full submitted form
// lives in field_values so adding modules needs no schema change.
type ModuleRecord struct {
	ID        string         `json:"id"`
	Module    string         `json:"module"`
	Name      string         `json:"name"`
	Owner     string         `json:"owner"`
	Status    string         `json:"status"`
	Values    map[string]any `json:"values"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type ModuleRecordInput struct {
	Name   string         `json:"name"`
	Owner  string         `json:"owner"`
	Status string         `json:"status"`
	Values map[string]any `json:"values"`
}

func scanModuleRecord(row pgx.Row) (ModuleRecord, error) {
	var m ModuleRecord
	var raw []byte
	if err := row.Scan(&m.ID, &m.Module, &m.Name, &m.Owner, &m.Status, &raw, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return m, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m.Values)
	}
	if m.Values == nil {
		m.Values = map[string]any{}
	}
	return m, nil
}

const moduleRecordCols = "id, module, name, owner, status, field_values, created_at, updated_at"

func (r *Repository) ListModuleRecords(ctx context.Context, module string, opts ListOpts) ([]ModuleRecord, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	// Custom modules (products, projects, documents, …) carry an owner like any
	// other record, so they get the same enforced scope. This list used to be
	// built from the unscoped listOpts, which meant a sales rep saw every custom
	// record in the business even while their accounts and deals were scoped.
	where := []string{"module = $1"}
	args := []any{module}
	i := 2
	where, args = applyScope(opts, "owner", where, args, &i)
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db(ctx).QueryRow(ctx,
		`SELECT COUNT(*)::int FROM crm_module_records WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db(ctx).Query(ctx, `SELECT `+moduleRecordCols+`
		FROM crm_module_records WHERE `+whereSQL+`
		ORDER BY created_at DESC LIMIT $`+fmt.Sprint(i)+` OFFSET $`+fmt.Sprint(i+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ModuleRecord{}
	for rows.Next() {
		m, err := scanModuleRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetModuleRecord(ctx context.Context, module, id string) (ModuleRecord, error) {
	row := r.db(ctx).QueryRow(ctx, `SELECT `+moduleRecordCols+`
		FROM crm_module_records WHERE id = $1 AND module = $2`, id, module)
	return scanModuleRecord(row)
}

func (r *Repository) CreateModuleRecord(ctx context.Context, module string, in ModuleRecordInput) (ModuleRecord, error) {
	id, err := r.NextID(ctx, "MREC", 1000)
	if err != nil {
		return ModuleRecord{}, err
	}
	valuesRaw, _ := json.Marshal(coalesceMap(in.Values))
	now := time.Now().UTC()
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_module_records (id, module, name, owner, status, field_values, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
	`, id, module, in.Name, in.Owner, in.Status, valuesRaw, now)
	if err != nil {
		return ModuleRecord{}, err
	}
	return r.GetModuleRecord(ctx, module, id)
}

// UpdateModuleRecord replaces the display fields and form blob for a record. The
// UI submits the full form on edit, so this is a whole-record update rather than
// a sparse patch.
func (r *Repository) UpdateModuleRecord(ctx context.Context, module, id string, in ModuleRecordInput) (ModuleRecord, error) {
	valuesRaw, _ := json.Marshal(coalesceMap(in.Values))
	tag, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_module_records
		SET name = $3, owner = $4, status = $5, field_values = $6, updated_at = NOW()
		WHERE id = $1 AND module = $2
	`, id, module, in.Name, in.Owner, in.Status, valuesRaw)
	if err != nil {
		return ModuleRecord{}, err
	}
	if tag.RowsAffected() == 0 {
		return ModuleRecord{}, pgx.ErrNoRows
	}
	return r.GetModuleRecord(ctx, module, id)
}

func (r *Repository) DeleteModuleRecord(ctx context.Context, module, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_module_records WHERE id = $1 AND module = $2`, id, module)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func coalesceMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
