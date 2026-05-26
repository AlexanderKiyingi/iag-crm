package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListBridgeStreams(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, direction, status, last_sync_at, record_count FROM crm_bridge_streams ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name, dir, status string
		var last *time.Time
		var count int
		if err := rows.Scan(&id, &name, &dir, &status, &last, &count); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "name": name, "direction": dir, "status": status,
			"last_sync_at": last, "record_count": count,
		})
	}
	return out, rows.Err()
}

func (r *Repository) AssignPendingImport(ctx context.Context, id, owner string) (map[string]any, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE crm_bridge_pending_imports
		SET assigned_owner = $2, status = 'assigned'
		WHERE id = $1 AND status = 'pending'
	`, id, owner)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id, dms_outlet_id, channel, beat, assigned_owner, status, created_at
		FROM crm_bridge_pending_imports WHERE id = $1
	`, id)
	var pid, outlet, channel, beat, assigned, status string
	var created time.Time
	if err := row.Scan(&pid, &outlet, &channel, &beat, &assigned, &status, &created); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": pid, "dms_outlet_id": outlet, "channel": channel, "beat": beat,
		"assigned_owner": assigned, "status": status, "created_at": created,
	}, nil
}

func (r *Repository) ListPendingImports(ctx context.Context) ([]map[string]any, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_bridge_pending_imports WHERE status = 'pending'`).Scan(&total)
	rows, err := r.pool.Query(ctx, `
		SELECT id, dms_outlet_id, channel, beat, assigned_owner, status, created_at
		FROM crm_bridge_pending_imports WHERE status = 'pending' ORDER BY created_at DESC LIMIT 50
	`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, outlet, channel, beat, owner, status string
		var created time.Time
		if err := rows.Scan(&id, &outlet, &channel, &beat, &owner, &status, &created); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]any{
			"id": id, "dms_outlet_id": outlet, "channel": channel, "beat": beat,
			"assigned_owner": owner, "status": status, "created_at": created,
		})
	}
	return out, total, nil
}

func (r *Repository) ListFieldMappings(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, stream_id, dms_field, crm_field, transform FROM crm_bridge_field_mappings ORDER BY stream_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, stream, dms, crm, transform string
		if err := rows.Scan(&id, &stream, &dms, &crm, &transform); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "stream_id": stream, "dms_field": dms, "crm_field": crm, "transform": transform,
		})
	}
	return out, rows.Err()
}

func (r *Repository) ListSyncLog(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, stream_id, level, message, created_at FROM crm_bridge_sync_log ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, stream, level, msg string
		var created time.Time
		if err := rows.Scan(&id, &stream, &level, &msg, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "stream_id": stream, "level": level, "message": msg, "created_at": created,
		})
	}
	return out, rows.Err()
}

func (r *Repository) ListOutlets(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_outlets", opts, "name, dms_ref, city, segment, health, owner")
}

func (r *Repository) GetOutlet360(ctx context.Context, id string) (map[string]any, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.dms_ref, o.city, o.segment, o.health, o.owner, a.name
		FROM crm_outlets o
		LEFT JOIN crm_accounts a ON a.id = o.account_id
		WHERE o.id = $1 OR o.dms_ref = $1
	`, id)
	var oid, name, dms, city, segment, owner, account string
	var health int
	if err := row.Scan(&oid, &name, &dms, &city, &segment, &health, &owner, &account); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": oid, "name": name, "dms_ref": dms, "city": city, "segment": segment,
		"health": health, "owner": owner, "account": account,
		"orders": []map[string]any{{"ref": "ORD-8842", "amount": "UGX 8.4M", "at": "2026-05-02"}},
		"tickets": []map[string]any{{"id": "TKT-0084", "subject": "Packaging tear"}},
		"loyalty": map[string]any{"tier": "Gold", "points": 12400},
	}, nil
}

func (r *Repository) ListExportCustomers(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_export_customers", opts, "name, country, currency, incoterms, credit_limit, erp_ref, status")
}

func (r *Repository) CreateExportCustomer(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "EXP", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_export_customers (id, name, country, currency, incoterms, credit_limit, erp_ref, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
	`, id, str(in, "name"), str(in, "country"), str(in, "currency"), str(in, "incoterms"), num(in, "credit_limit"), str(in, "erp_ref"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListLoyaltyTiers(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, min_points, benefits, multiplier FROM crm_loyalty_tiers ORDER BY min_points`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name, benefits string
		var minPts int
		var mult float64
		if err := rows.Scan(&id, &name, &minPts, &benefits, &mult); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "name": name, "min_points": minPts, "benefits": benefits, "multiplier": mult})
	}
	return out, rows.Err()
}

func (r *Repository) ListLoyaltyOutlets(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_loyalty_outlets", opts, "name, dms_ref, tier_id, points, status")
}

func (r *Repository) CreateLoyaltyPromotion(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "PRM", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_loyalty_promotions (id, name, tier_id, discount, status, created_at)
		VALUES ($1,$2,$3,$4,'draft',NOW())
	`, id, str(in, "name"), str(in, "tier_id"), str(in, "discount"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) InsightsSummary(ctx context.Context) (map[string]any, error) {
	var atRisk int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_accounts WHERE status IN ('risk','nurture')`).Scan(&atRisk)
	return map[string]any{
		"churn_risk_accounts": atRisk,
		"nrr":                 114,
		"cohort_retention":    71,
		"at_risk_exposure_usd": 182000,
	}, nil
}

func (r *Repository) AISuggestions(ctx context.Context) ([]map[string]any, error) {
	return []map[string]any{
		{"type": "deal_risk", "title": "Review Seoul Bean Lab", "deal_id": "DEAL-0419", "priority": "high"},
		{"type": "next_action", "title": "Schedule call · Matsuri Q3", "deal_id": "DEAL-0421", "priority": "medium"},
		{"type": "email_draft", "title": "Renewal nudge · Union Hand-Roasted", "account_id": "ACC-0412", "priority": "low"},
	}, nil
}
