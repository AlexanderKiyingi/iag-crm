package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type LookupItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Extra string `json:"extra,omitempty"`
}

func (r *Repository) Lookups(ctx context.Context, kind string) ([]LookupItem, error) {
	switch kind {
	case "accounts":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(segment,'') FROM crm_accounts ORDER BY name LIMIT 200`)
	case "contacts":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(account_name,'') FROM crm_contacts ORDER BY name LIMIT 200`)
	case "deals":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(account_name,'') FROM crm_deals ORDER BY updated_at DESC LIMIT 200`)
	case "segments":
		return r.lookupQuery(ctx, `SELECT id, name, '' FROM crm_segments ORDER BY name LIMIT 200`)
	case "campaigns":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(campaign_type,'') FROM crm_campaigns ORDER BY name LIMIT 200`)
	case "journeys":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(trigger,'') FROM crm_journeys ORDER BY name LIMIT 200`)
	case "personas":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(buyer_role,'') FROM crm_personas ORDER BY name LIMIT 200`)
	case "events":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(event_type,'') FROM crm_events ORDER BY name LIMIT 200`)
	case "outlets":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(dms_ref,'') FROM crm_outlets ORDER BY name LIMIT 200`)
	case "contentLib":
		return r.lookupQuery(ctx, `SELECT id, name, COALESCE(asset_type,'') FROM crm_content_assets ORDER BY name LIMIT 200`)
	default:
		return nil, fmt.Errorf("unknown lookup kind: %s", kind)
	}
}

func (r *Repository) lookupQuery(ctx context.Context, q string) ([]LookupItem, error) {
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LookupItem
	for rows.Next() {
		var it LookupItem
		if err := rows.Scan(&it.ID, &it.Label, &it.Extra); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

type NamedRow struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Extra     map[string]any `json:"extra,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (r *Repository) listNamed(ctx context.Context, table string, extraCols string, limit, offset int) ([]NamedRow, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*)::int FROM "+table).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`SELECT id, name, %s, created_at, updated_at FROM %s ORDER BY created_at DESC LIMIT $1 OFFSET $2`, extraCols, table)
	rows, err := r.pool.Query(ctx, q, clampLimit(limit), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []NamedRow
	for rows.Next() {
		var row NamedRow
		var extraRaw []byte
		if err := rows.Scan(&row.ID, &row.Name, &extraRaw, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(extraRaw, &row.Extra)
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func (r *Repository) createNamed(ctx context.Context, table, prefix string, start int64, name string, extraJSON []byte) (NamedRow, error) {
	id, err := r.NextID(ctx, prefix, start)
	if err != nil {
		return NamedRow{}, err
	}
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, name, extra, created_at, updated_at) VALUES ($1,$2,$3,$4,$4)`, table),
		id, name, extraJSON, now)
	if err != nil {
		// fallback for tables without generic extra column
		_, err = r.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (id, name, created_at, updated_at) VALUES ($1,$2,$3,$3)`, table), id, name, now)
		if err != nil {
			return NamedRow{}, err
		}
	}
	return NamedRow{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}

func scanNamedExtra(row pgx.Row) (NamedRow, error) {
	var n NamedRow
	var extra []byte
	err := row.Scan(&n.ID, &n.Name, &extra, &n.CreatedAt, &n.UpdatedAt)
	if err == nil {
		_ = json.Unmarshal(extra, &n.Extra)
	}
	return n, err
}

// --- Segments ---
func (r *Repository) ListSegments(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_segments", opts, "name, kind, refresh, rules, member_count")
}

func (r *Repository) CreateSegment(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "SEG", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_segments (id, name, kind, refresh, rules, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
	`, id, str(in, "name"), str(in, "kind"), str(in, "refresh"), str(in, "rules"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": str(in, "name")}, nil
}

func (r *Repository) ListJourneys(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_journeys", opts, "name, trigger, template, goal, status, enrolled, conversion")
}

func (r *Repository) CreateJourney(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "JRN", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_journeys (id, name, trigger, template, goal, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
	`, id, str(in, "name"), str(in, "trigger"), str(in, "template"), str(in, "goal"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListPersonas(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_personas", opts, "name, buyer_role, region, seniority, content_tags, story")
}

func (r *Repository) CreatePersona(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "PSN", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_personas (id, name, buyer_role, region, seniority, content_tags, story, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())
	`, id, str(in, "name"), str(in, "buyer_role"), str(in, "region"), str(in, "seniority"), str(in, "content_tags"), str(in, "story"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListEvents(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_events", opts, "name, event_type, city, starts_on, ends_on, budget_usd, mql_target, registrations, status")
}

func (r *Repository) CreateEvent(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "EVT", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_events (id, name, event_type, city, budget_usd, mql_target, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
	`, id, str(in, "name"), str(in, "type"), str(in, "city"), num(in, "budget_usd"), intNum(in, "mql_target"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListContentAssets(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_content_assets", opts, "name, asset_type, format, tags, status, usage_count")
}

func (r *Repository) CreateContentAsset(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "AST", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_content_assets (id, name, asset_type, format, tags, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
	`, id, str(in, "name"), str(in, "type"), str(in, "format"), str(in, "tags"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListEmailSends(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_email_sends", opts, "subject, template, status, scheduled_at, open_rate")
}

func (r *Repository) CreateEmailSend(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "EML", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_email_sends (id, subject, template, body, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'draft',NOW(),NOW())
	`, id, str(in, "subject"), str(in, "template"), str(in, "body"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListSocialPosts(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_social_posts", opts, "platforms, content, status, scheduled_at")
}

func (r *Repository) CreateSocialPost(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "SOC", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_social_posts (id, platforms, content, status, created_at, updated_at)
		VALUES ($1,$2,$3,'draft',NOW(),NOW())
	`, id, str(in, "platforms"), str(in, "content"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListSEOKeywords(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_seo_keywords", opts, "term, intent, locale, landing_page, rank")
}

func (r *Repository) CreateSEOKeyword(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "KW", 100)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_seo_keywords (id, term, intent, locale, landing_page, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
	`, id, str(in, "term"), str(in, "intent"), str(in, "locale"), str(in, "landing_page"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListBudgetPlans(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_budget_plans", opts, "name, quarter, owner, channels, mql_target, sql_target, won_target")
}

func (r *Repository) CreateBudgetPlan(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "BDG", 100)
	ch, _ := json.Marshal(in["channels"])
	_, err := r.pool.Exec(ctx, `
		INSERT INTO crm_budget_plans (id, name, quarter, owner, channels, mql_target, sql_target, won_target, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
	`, id, str(in, "name"), str(in, "quarter"), str(in, "owner"), ch, intNum(in, "mql_target"), intNum(in, "sql_target"), num(in, "won_target"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListMQLs(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_mqls", opts, "name, company, score, source, owner, status")
}

func (r *Repository) GetBrandKit(ctx context.Context) (map[string]any, error) {
	row := r.pool.QueryRow(ctx, `SELECT colors, fonts, voice, updated_at FROM crm_brand_kit WHERE id = 'default'`)
	var colors, fonts []byte
	var voice string
	var updated time.Time
	if err := row.Scan(&colors, &fonts, &voice, &updated); err != nil {
		if err == pgx.ErrNoRows {
			return map[string]any{
				"colors": map[string]string{"crm": "#7C3AED", "primary": "#7C3AED"},
				"fonts":  map[string]string{"display": "Fraunces", "body": "DM Sans"},
				"voice":  "Specialty coffee · traceable · partner-first",
			}, nil
		}
		return nil, err
	}
	var c, f map[string]any
	_ = json.Unmarshal(colors, &c)
	_ = json.Unmarshal(fonts, &f)
	return map[string]any{"colors": c, "fonts": f, "voice": voice, "updated_at": updated}, nil
}

func (r *Repository) MarketingHubSummary(ctx context.Context) (map[string]any, error) {
	return map[string]any{
		"campaigns_live": 3, "mqls_this_month": 142, "content_assets": 28,
		"budget_burn_pct": 68, "upcoming_events": 2,
	}, nil
}

func (r *Repository) DemandGenMetrics(ctx context.Context) (map[string]any, error) {
	return map[string]any{
		"channels": []map[string]any{
			{"name": "Email", "cpl": 42, "mqls": 86},
			{"name": "Trade shows", "cpl": 118, "mqls": 54},
			{"name": "LinkedIn", "cpl": 64, "mqls": 32},
		},
	}, nil
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func num(m map[string]any, k string) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

func intNum(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func (r *Repository) listGeneric(ctx context.Context, table string, opts ListOpts, cols string) ([]map[string]any, int, error) {
	opts.Limit = clampLimit(opts.Limit)
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*)::int FROM "+table).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`
		SELECT row_to_json(x)::text FROM (
			SELECT id, %s, created_at, updated_at FROM %s ORDER BY created_at DESC LIMIT $1 OFFSET $2
		) x`, cols, table)
	rows, err := r.pool.Query(ctx, q, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, 0, err
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
