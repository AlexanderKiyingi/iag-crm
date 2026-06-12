package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetGenericRow fetches a marketing/bridge row by id.
func (r *Repository) GetGenericRow(ctx context.Context, table, cols, id string) (map[string]any, error) {
	q := fmt.Sprintf(`
		SELECT row_to_json(x)::text FROM (
			SELECT id, %s, created_at, updated_at FROM %s WHERE id = $1
		) x`, cols, table)
	var raw string
	if err := r.db(ctx).QueryRow(ctx, q, id).Scan(&raw); err != nil {
		return nil, err
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		return nil, err
	}
	return row, nil
}

// PatchGenericRow updates allowed columns on a marketing table row.
func (r *Repository) PatchGenericRow(ctx context.Context, table string, id string, patch map[string]any, allowed map[string]string) (map[string]any, error) {
	sets := []string{"updated_at = NOW()"}
	args := []any{id}
	i := 2
	for key, col := range allowed {
		if v, ok := patch[key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", col, i))
			args = append(args, v)
			i++
		}
	}
	if len(sets) == 1 {
		cols := ""
		for _, c := range allowed {
			if cols != "" {
				cols += ", "
			}
			cols += c
		}
		return r.GetGenericRow(ctx, table, cols, id)
	}
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = $1", table, stringsJoin(sets, ", "))
	tag, err := r.db(ctx).Exec(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	cols := ""
	for _, c := range allowed {
		if cols != "" {
			cols += ", "
		}
		cols += c
	}
	return r.GetGenericRow(ctx, table, cols, id)
}

// DeleteGenericRow removes a marketing table row.
func (r *Repository) DeleteGenericRow(ctx context.Context, table, id string) error {
	tag, err := r.db(ctx).Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = $1", table), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
