package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/dmsclient"
	"github.com/iag/crm/backend/internal/models"
)

// UpsertOutletFromDMS syncs a DMS outlet into crm_outlets; returns upserted, pendingCreated.
func (r *Repository) UpsertOutletFromDMS(ctx context.Context, o dmsclient.Outlet) (bool, bool, error) {
	var exists bool
	_ = r.db(ctx).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crm_outlets WHERE dms_ref = $1)`, o.ID).Scan(&exists)
	if exists {
		_, err := r.db(ctx).Exec(ctx, `
			UPDATE crm_outlets SET name = $2, city = $3, segment = $4, health = $5, updated_at = NOW()
			WHERE dms_ref = $1
		`, o.ID, o.Name, o.Address, o.Channel, scoreToHealth(o.Score))
		return false, false, err
	}
	id := r.NewID()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_outlets (id, name, dms_ref, city, segment, health, owner, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'',NOW(),NOW())
	`, id, o.Name, o.ID, o.Address, o.Channel, scoreToHealth(o.Score))
	if err != nil {
		return false, false, err
	}
	pending, err := r.EnsurePendingImport(ctx, o.ID, o.Name)
	return true, pending, err
}

func scoreToHealth(score string) int {
	switch score {
	case "A", "Gold":
		return 90
	case "B":
		return 75
	case "C":
		return 55
	default:
		return 70
	}
}

// EnsurePendingImport creates a pending import row when missing.
func (r *Repository) EnsurePendingImport(ctx context.Context, dmsOutletID, name string) (bool, error) {
	var exists bool
	_ = r.db(ctx).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crm_bridge_pending_imports WHERE dms_outlet_id = $1)`, dmsOutletID).Scan(&exists)
	if exists {
		return false, nil
	}
	id := r.NewID()
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_bridge_pending_imports (id, dms_outlet_id, channel, beat, status, created_at)
		VALUES ($1,$2,'DMS','auto','pending',NOW())
	`, id, dmsOutletID)
	return true, err
}

func (r *Repository) TouchBridgeStreams(ctx context.Context) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_bridge_streams SET last_sync_at = NOW(), record_count = record_count + 1`)
	return err
}

func (r *Repository) SetDealFinanceARRef(ctx context.Context, dealID, ref string) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_deals SET finance_ar_ref = $2, updated_at = NOW() WHERE id = $1`, dealID, ref)
	return err
}

func (r *Repository) SetQuoteFinanceARRef(ctx context.Context, quoteID, ref string) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_quotes SET finance_ar_ref = $2, updated_at = NOW() WHERE id = $1`, quoteID, ref)
	return err
}

func (r *Repository) SetQuoteContractRef(ctx context.Context, quoteID, contractNo string) error {
	_, err := r.db(ctx).Exec(ctx, `UPDATE crm_quotes SET contract_ref = $2, updated_at = NOW() WHERE id = $1 OR ref = $1`, quoteID, contractNo)
	return err
}

func (r *Repository) DeleteDeal(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_deals WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteLead(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_leads WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteQuote(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_quotes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteTicket(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_tickets WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteCampaign(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_campaigns WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) PatchActivity(ctx context.Context, id string, patch map[string]any) (models.Activity, error) {
	sets := []string{}
	args := []any{id}
	i := 2
	for _, field := range []struct{ key, col string }{
		{"subject", "subject"}, {"body", "body"}, {"type", "activity_type"}, {"owner", "owner"},
		// status was not a column at all, so a follow-up could never be marked
		// Done — it stayed Planned for ever no matter what the operator clicked.
		{"status", "status"},
	} {
		if v, ok := patch[field.key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.col, i))
			args = append(args, v)
			i++
		}
	}
	for _, field := range []struct{ key, col string }{
		// occurred_at was create-only; due_at did not exist.
		{"occurred_at", "occurred_at"}, {"due_at", "due_at"},
	} {
		if v, ok := patch[field.key]; ok {
			sets = append(sets, fmt.Sprintf("%s = $%d", field.col, i))
			args = append(args, parsePatchTime(v))
			i++
		}
	}
	if attrs, ok := patchAttrs(patch); ok {
		sets = append(sets, fmt.Sprintf("attrs = $%d", i))
		args = append(args, encodeAttrs(attrs))
		i++
	}
	if len(sets) == 0 {
		return r.GetActivity(ctx, id)
	}
	q := fmt.Sprintf("UPDATE crm_activities SET %s WHERE id = $1", stringsJoin(sets, ", "))
	tag, err := r.db(ctx).Exec(ctx, q, args...)
	if err != nil {
		return models.Activity{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Activity{}, pgx.ErrNoRows
	}
	return r.GetActivity(ctx, id)
}

func (r *Repository) DeleteActivity(ctx context.Context, id string) error {
	tag, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_activities WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) GetActivity(ctx context.Context, id string) (models.Activity, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, activity_type, subject, body, account_id, account_name, contact_id, deal_id,
		       outlet_ref, owner, occurred_at, due_at, status, attrs, created_at
		FROM crm_activities WHERE id = $1
	`, id)
	return scanActivity(row)
}

func stringsJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

// splitCSV splits a comma-separated value into trimmed, non-empty parts.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// GlobalSearch finds entities matching q across core CRM tables.
func (r *Repository) GlobalSearch(ctx context.Context, q string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 25
	}
	pattern := "%" + q + "%"
	rows, err := r.db(ctx).Query(ctx, `
		SELECT kind, id, label, extra FROM (
			SELECT 'account' AS kind, id, name AS label, segment AS extra FROM crm_accounts WHERE name ILIKE $1 OR segment ILIKE $1
			UNION ALL
			SELECT 'contact', id, name, email FROM crm_contacts WHERE name ILIKE $1 OR email ILIKE $1
			UNION ALL
			SELECT 'deal', id, name, account_name FROM crm_deals WHERE name ILIKE $1 OR account_name ILIKE $1
			UNION ALL
			SELECT 'lead', id, name, company FROM crm_leads WHERE name ILIKE $1 OR company ILIKE $1
			UNION ALL
			SELECT 'quote', id, ref, account_name FROM crm_quotes WHERE ref ILIKE $1 OR account_name ILIKE $1
		) hits ORDER BY label LIMIT $2
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var kind, id, label, extra string
		if err := rows.Scan(&kind, &id, &label, &extra); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"kind": kind, "id": id, "label": label, "extra": extra})
	}
	return out, rows.Err()
}

func (r *Repository) CreateExportJob(ctx context.Context, id, page, rangeKey, formats string) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_export_jobs (id, page, range_key, formats, status, created_at)
		VALUES ($1,$2,$3,$4,'queued',NOW())
	`, id, page, rangeKey, formats)
	return err
}

func (r *Repository) CompleteExportJob(ctx context.Context, id string) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_export_jobs SET status = 'completed', completed_at = NOW(),
			result_url = '/exports/' || id || '.zip'
		WHERE id = $1
	`, id)
	return err
}

func (r *Repository) GetExportJob(ctx context.Context, id string) (map[string]any, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, page, range_key, formats, status, COALESCE(result_url,''), created_at, completed_at
		FROM crm_export_jobs WHERE id = $1
	`, id)
	var jobID, page, rangeKey, formats, status, resultURL string
	var created time.Time
	var completed *time.Time
	if err := row.Scan(&jobID, &page, &rangeKey, &formats, &status, &resultURL, &created, &completed); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": jobID, "page": page, "range": rangeKey, "formats": formats,
		"status": status, "result_url": resultURL, "created_at": created, "completed_at": completed,
	}, nil
}

func (r *Repository) CreateSEOAuditJob(ctx context.Context, id, url string) error {
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_seo_audit_jobs (id, url, status, created_at) VALUES ($1,$2,'running',NOW())
	`, id, url)
	return err
}

func (r *Repository) CompleteSEOAuditJob(ctx context.Context, id string, score int, findings any) error {
	raw, _ := json.Marshal(findings)
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_seo_audit_jobs SET status = 'completed', score = $2, findings = $3::jsonb, completed_at = NOW()
		WHERE id = $1
	`, id, score, raw)
	return err
}

func (r *Repository) GetSEOAuditJob(ctx context.Context, id string) (map[string]any, error) {
	row := r.db(ctx).QueryRow(ctx, `
		SELECT id, url, status, score, findings, created_at, completed_at FROM crm_seo_audit_jobs WHERE id = $1
	`, id)
	var jobID, url, status string
	var score *int
	var findings []byte
	var created time.Time
	var completed *time.Time
	if err := row.Scan(&jobID, &url, &status, &score, &findings, &created, &completed); err != nil {
		return nil, err
	}
	var parsed any
	_ = json.Unmarshal(findings, &parsed)
	out := map[string]any{
		"id": jobID, "url": url, "status": status, "created_at": created, "completed_at": completed,
	}
	if score != nil {
		out["score"] = *score
	}
	if parsed != nil {
		out["findings"] = parsed
	}
	return out, nil
}

// LiveNotifications builds notifications from recent CRM activity.
func (r *Repository) LiveNotifications(ctx context.Context, limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = 9
	}
	rows, err := r.db(ctx).Query(ctx, `
		SELECT activity_type, subject, owner, occurred_at FROM crm_activities
		ORDER BY occurred_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Notification{}
	for rows.Next() {
		var kind, subject, owner string
		var at time.Time
		if err := rows.Scan(&kind, &subject, &owner, &at); err != nil {
			return nil, err
		}
		out = append(out, models.Notification{
			Kind: "crm", Icon: "•", Title: subject, Body: owner, Time: relativeTouch(at), Go: "activities",
		})
	}
	return out, nil
}

// ComputeOverviewMetrics derives dashboard metrics from live data.
func (r *Repository) ComputeOverviewMetrics(ctx context.Context, rangeKey string) (models.RangeMetrics, error) {
	summary, err := r.PipelineSummary(ctx)
	if err != nil {
		return models.RangeMetrics{}, err
	}
	var outlets int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_outlets`).Scan(&outlets)
	weighted, _ := summary["pipeline_weighted"].(float64)
	winRate, _ := summary["win_rate"].(float64)
	velocity, _ := summary["velocity_days"].(int)
	if rangeKey == "" {
		rangeKey = "week"
	}
	metrics := models.RangeMetrics{
		Label: overviewRangeLabel(rangeKey), Pipeline: fmt.Sprintf("$%.0fK", weighted/1000), PipelineDelta: "live",
		Outlets: fmt.Sprintf("%d", outlets), WinRate: fmt.Sprintf("%.1f", winRate),
		Cycle: fmt.Sprintf("%d", velocity), NRR: "—",
	}
	series, labels, err := r.pipelineSeries(ctx, rangeKey)
	if err == nil {
		metrics = mergeOverviewMetrics(metrics, series, labels)
	} else {
		// The headline figures above are real. A fabricated series was grafted on
		// here when pipelineSeries failed, which left invented movement sitting
		// beside live totals in one payload with nothing to tell them apart. The
		// series is now left empty and the client simply renders no chart.
		metrics.Series = []int{}
		metrics.XLabels = []string{}
	}
	return metrics, nil
}

// LiveBridgeStatus returns bridge status from database counts.
func (r *Repository) LiveBridgeStatus(ctx context.Context) map[string]any {
	var bridged, streams, pending int
	var lastSync *time.Time
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_accounts WHERE dms_bridged = TRUE`).Scan(&bridged)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_bridge_streams`).Scan(&streams)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_bridge_pending_imports WHERE status = 'pending'`).Scan(&pending)
	_ = r.db(ctx).QueryRow(ctx, `SELECT MAX(last_sync_at) FROM crm_bridge_streams`).Scan(&lastSync)
	last := "never"
	if lastSync != nil {
		last = relativeTouch(*lastSync)
	}
	return map[string]any{
		"connected":        streams > 0,
		"streams":          streams,
		"accounts_bridged": bridged,
		"last_sync_at":     last,
		"pending_imports":  pending,
	}
}
