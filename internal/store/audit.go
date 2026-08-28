package store

import (
	"context"
	"fmt"
	"time"

	platformdb "github.com/alvor-technologies/iag-platform-go/db"

	"github.com/iag/crm/backend/internal/models"
)

func (r *Repository) AppendAudit(ctx context.Context, action, detail, userName string) (models.AuditEntry, error) {
	id := r.NewID()
	now := time.Now().UTC()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_audit_entries (id, logged_at, user_name, action, detail)
		VALUES ($1, $2, $3, $4, $5)
	`, id, now, userName, action, detail)
	if err != nil {
		return models.AuditEntry{}, err
	}
	return models.AuditEntry{
		ID:       id,
		LoggedAt: now,
		UserName: userName,
		Action:   action,
		Detail:   detail,
	}, nil
}

func (r *Repository) ListAudit(ctx context.Context, limit int) ([]models.AuditEntry, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// Bounded: crm_audit_entries is append-only, so an unqualified COUNT(*) is
	// a full scan that grows every month to put a number beside at most 200
	// rows. A total equal to the cap means "at least this many".
	total, _, err := platformdb.CountBounded(ctx, r.db(ctx), platformdb.DefaultCountCap,
		"FROM crm_audit_entries")
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, logged_at, user_name, action, detail
		FROM crm_audit_entries ORDER BY logged_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.LoggedAt, &e.UserName, &e.Action, &e.Detail); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetAudit(ctx context.Context, id string) (models.AuditEntry, error) {
	var e models.AuditEntry
	err := r.db(ctx).QueryRow(ctx, `
		SELECT id, logged_at, user_name, action, detail
		FROM crm_audit_entries WHERE id = $1
	`, id).Scan(&e.ID, &e.LoggedAt, &e.UserName, &e.Action, &e.Detail)
	return e, err
}

func (r *Repository) LogAPIRequest(ctx context.Context, method, path string, statusCode int, userName string, durationMs int, clientIP string) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_api_audit (method, path, status_code, user_name, duration_ms, client_ip)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, method, path, statusCode, userName, durationMs, clientIP)
	return err
}

func (r *Repository) APIMonitoringSummary(ctx context.Context) (map[string]any, error) {
	var total24h, errors24h int
	var avgMs float64
	err := r.db(ctx).QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status_code >= 400)::int,
			COALESCE(AVG(duration_ms), 0)
		FROM crm_api_audit
		WHERE logged_at >= NOW() - INTERVAL '24 hours'
	`).Scan(&total24h, &errors24h, &avgMs)
	if err != nil {
		return nil, err
	}
	var bridgePending int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_bridge_pending_imports WHERE status = 'pending'`).Scan(&bridgePending)
	return map[string]any{
		"requests_24h":       total24h,
		"errors_24h":         errors24h,
		"avg_duration_ms":    avgMs,
		"bridge_pending":     bridgePending,
		"event_bus_enabled":  false,
	}, nil
}

func (r *Repository) APIMonitoringActivity(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT method, path, status_code, user_name, duration_ms, logged_at
		FROM crm_api_audit ORDER BY logged_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var method, path, user string
		var status, dur int
		var at time.Time
		if err := rows.Scan(&method, &path, &status, &user, &dur, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"method": method, "path": path, "status": status,
			"user": user, "duration_ms": dur, "logged_at": at,
		})
	}
	return out, rows.Err()
}

func (r *Repository) ListAPIAuditLogs(ctx context.Context, limit int) ([]map[string]any, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// Bounded: crm_api_audit takes a row per API request, so this is the
	// fastest-growing table in the service.
	total, _, err := platformdb.CountBounded(ctx, r.db(ctx), platformdb.DefaultCountCap,
		"FROM crm_api_audit")
	if err != nil {
		return nil, 0, err
	}
	items, err := r.APIMonitoringActivity(ctx, limit)
	return items, total, err
}

func (r *Repository) ListBuyingSignals(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT id, account_id, account_name, signal_type, strength, action_hint, observed_at
		FROM crm_buying_signals ORDER BY observed_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, acctName, sigType, strength, action string
		// account_id is nullable (ON DELETE SET NULL); scanning NULL into a plain
		// string errors, which surfaced as a 500 on GET /insights/signals.
		var acctID *string
		var at time.Time
		if err := rows.Scan(&id, &acctID, &acctName, &sigType, &strength, &action, &at); err != nil {
			return nil, err
		}
		accountID := ""
		if acctID != nil {
			accountID = *acctID
		}
		out = append(out, map[string]any{
			"id": id, "account_id": accountID, "account": acctName,
			"signal": sigType, "strength": strength, "action": action, "observed_at": at,
		})
	}
	return out, rows.Err()
}

func (r *Repository) GetLoyaltyTierRules(ctx context.Context) (map[string]any, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, name, bronze_min_orders, bronze_min_revenue, silver_min_orders, silver_min_revenue,
		       gold_min_orders, gold_min_revenue, updated_at, updated_by
		FROM crm_loyalty_tier_rules WHERE id = 'default'
	`)
	var id, name, updatedBy string
	var bOrd, sOrd, gOrd int
	var bRev, sRev, gRev float64
	var updatedAt time.Time
	if err := row.Scan(&id, &name, &bOrd, &bRev, &sOrd, &sRev, &gOrd, &gRev, &updatedAt, &updatedBy); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "name": name,
		"bronze": map[string]any{"min_orders": bOrd, "min_revenue_m": bRev},
		"silver": map[string]any{"min_orders": sOrd, "min_revenue_m": sRev},
		"gold":   map[string]any{"min_orders": gOrd, "min_revenue_m": gRev},
		"updated_at": updatedAt, "updated_by": updatedBy,
	}, nil
}

func (r *Repository) PutLoyaltyTierRules(ctx context.Context, in map[string]any, userName string) (map[string]any, error) {
	name := strField(in, "name", "Pearl Club v2 · 2026")
	bOrd := intField(in, "bronze_min_orders", 2)
	bRev := floatField(in, "bronze_min_revenue", 2)
	sOrd := intField(in, "silver_min_orders", 4)
	sRev := floatField(in, "silver_min_revenue", 6)
	gOrd := intField(in, "gold_min_orders", 8)
	gRev := floatField(in, "gold_min_revenue", 14)
	if bronze, ok := in["bronze"].(map[string]any); ok {
		bOrd = intField(bronze, "min_orders", bOrd)
		bRev = floatField(bronze, "min_revenue_m", bRev)
	}
	if silver, ok := in["silver"].(map[string]any); ok {
		sOrd = intField(silver, "min_orders", sOrd)
		sRev = floatField(silver, "min_revenue_m", sRev)
	}
	if gold, ok := in["gold"].(map[string]any); ok {
		gOrd = intField(gold, "min_orders", gOrd)
		gRev = floatField(gold, "min_revenue_m", gRev)
	}
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_loyalty_tier_rules SET
			name = $1, bronze_min_orders = $2, bronze_min_revenue = $3,
			silver_min_orders = $4, silver_min_revenue = $5,
			gold_min_orders = $6, gold_min_revenue = $7,
			updated_at = NOW(), updated_by = $8
		WHERE id = 'default'
	`, name, bOrd, bRev, sOrd, sRev, gOrd, gRev, userName)
	if err != nil {
		return nil, err
	}
	return r.GetLoyaltyTierRules(ctx)
}

func strField(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func intField(m map[string]any, key string, fallback int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return fallback
	}
}

func floatField(m map[string]any, key string, fallback float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return fallback
}

func (r *Repository) MonitoringSummaryWithBus(ctx context.Context, busEnabled bool) (map[string]any, error) {
	summary, err := r.APIMonitoringSummary(ctx)
	if err != nil {
		return nil, err
	}
	summary["event_bus_enabled"] = busEnabled
	return summary, nil
}

func (r *Repository) AppendBridgeSyncLog(ctx context.Context, message string) error {
	id := r.NewID()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_bridge_sync_log (id, stream_id, level, message, created_at)
		VALUES ($1, 'all', 'info', $2, NOW())
	`, id, message)
	return err
}

func AuditDetail(entityType, entityID, verb string) string {
	return fmt.Sprintf("%s %s %s", verb, entityType, entityID)
}
