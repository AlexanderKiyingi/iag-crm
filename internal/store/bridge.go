package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListBridgeStreams(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db(ctx).Query(ctx, `SELECT id, name, direction, status, last_sync_at, record_count FROM crm_bridge_streams ORDER BY name`)
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
	tag, err := r.db(ctx).Exec(ctx, `
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
	row := r.db(ctx).QueryRow(ctx, `
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
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_bridge_pending_imports WHERE status = 'pending'`).Scan(&total)
	rows, err := r.db(ctx).Query(ctx, `
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
	rows, err := r.db(ctx).Query(ctx, `SELECT id, stream_id, dms_field, crm_field, transform FROM crm_bridge_field_mappings ORDER BY stream_id`)
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
	rows, err := r.db(ctx).Query(ctx, `
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

func (r *Repository) GetOutlet(ctx context.Context, id string) (map[string]any, error) {
	return r.GetGenericRow(ctx, "crm_outlets", "name, dms_ref, city, segment, health, owner", id)
}

func (r *Repository) GetOutlet360(ctx context.Context, id string) (map[string]any, error) {
	row := r.db(ctx).QueryRow(ctx, `
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
	out := map[string]any{
		"id": oid, "name": name, "dms_ref": dms, "city": city, "segment": segment,
		"health": health, "owner": owner, "account": account,
	}
	tickets, _, _ := r.listGeneric(ctx, "crm_tickets", ListOpts{Limit: 5, Search: name}, "subject, status, priority, sla_due_at")
	out["tickets"] = tickets
	loyalty, _, _ := r.listGeneric(ctx, "crm_loyalty_outlets", ListOpts{Limit: 1, Search: dms}, "name, tier_id, points, status")
	if len(loyalty) > 0 {
		out["loyalty"] = loyalty[0]
	} else {
		out["loyalty"] = map[string]any{"tier": "—", "points": 0}
	}
	out["orders"] = []map[string]any{}
	return out, nil
}

func (r *Repository) ListExportCustomers(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_export_customers", opts, "name, country, currency, incoterms, credit_limit, erp_ref, status")
}

func (r *Repository) CreateExportCustomer(ctx context.Context, in map[string]any) (map[string]any, error) {
	id := r.NewID()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_export_customers (id, name, country, currency, incoterms, credit_limit, erp_ref, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
	`, id, str(in, "name"), str(in, "country"), str(in, "currency"), str(in, "incoterms"), num(in, "credit_limit"), str(in, "erp_ref"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListLoyaltyTiers(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db(ctx).Query(ctx, `SELECT id, name, min_points, benefits, multiplier FROM crm_loyalty_tiers ORDER BY min_points`)
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
	id := r.NewID()
	_, err := r.db(ctx).Exec(ctx, `
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
	var exposure float64
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_accounts WHERE status IN ('risk','nurture')`).Scan(&atRisk)
	_ = r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM crm_deals d
		JOIN crm_accounts a ON a.id = d.account_id
		WHERE a.status IN ('risk','nurture') AND d.stage NOT IN ('won','lost')
	`).Scan(&exposure)
	var won, total int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE stage='won'), COUNT(*) FROM crm_deals WHERE stage IN ('won','lost')`).Scan(&won, &total)
	retention := 0
	if total > 0 {
		retention = won * 100 / total
	}
	return map[string]any{
		"churn_risk_accounts":  atRisk,
		"nrr":                  retention,
		"cohort_retention":     retention,
		"at_risk_exposure_usd": exposure,
	}, nil
}

func (r *Repository) AISuggestions(ctx context.Context) ([]map[string]any, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, name, account_name, stage, amount FROM crm_deals
		WHERE stage IN ('negotiation','proposal') ORDER BY amount DESC LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	priority := []string{"high", "medium", "low"}
	i := 0
	for rows.Next() {
		var id, name, account, stage string
		var amount float64
		if err := rows.Scan(&id, &name, &account, &stage, &amount); err != nil {
			return nil, err
		}
		p := priority[i%len(priority)]
		out = append(out, map[string]any{
			"type": "deal_risk", "title": "Follow up: " + name, "deal_id": id,
			"account": account, "stage": stage, "priority": p,
		})
		i++
	}
	return out, rows.Err()
}
