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
	rows, err := r.db(ctx).Query(ctx, q)
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
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM "+table).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`SELECT id, name, %s, created_at, updated_at FROM %s ORDER BY created_at DESC LIMIT $1 OFFSET $2`, extraCols, table)
	rows, err := r.db(ctx).Query(ctx, q, clampLimit(limit), offset)
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
	_, err = r.db(ctx).Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, name, extra, created_at, updated_at) VALUES ($1,$2,$3,$4,$4)`, table),
		id, name, extraJSON, now)
	if err != nil {
		// fallback for tables without generic extra column
		_, err = r.db(ctx).Exec(ctx, fmt.Sprintf(`INSERT INTO %s (id, name, created_at, updated_at) VALUES ($1,$2,$3,$3)`, table), id, name, now)
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
	_, err := r.db(ctx).Exec(ctx, `
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
	_, err := r.db(ctx).Exec(ctx, `
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
	_, err := r.db(ctx).Exec(ctx, `
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
	_, err := r.db(ctx).Exec(ctx, `
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
	_, err := r.db(ctx).Exec(ctx, `
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
	status := str(in, "status")
	if status == "" {
		status = "draft"
	}
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_email_sends (id, subject, template, body, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW(),NOW())
	`, id, str(in, "subject"), str(in, "template"), str(in, "body"), status)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (r *Repository) ListSocialPosts(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	items, total, err := r.listGeneric(ctx, "crm_social_posts", opts, "platforms, content, status, scheduled_at")
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		normalizeSocialAliases(items[i])
	}
	return items, total, nil
}

func (r *Repository) CreateSocialPost(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "SOC", 100)
	platforms := str(in, "platforms")
	if platforms == "" {
		network := str(in, "network")
		handle := str(in, "handle")
		switch {
		case network != "" && handle != "":
			platforms = network + " / " + handle
		case network != "":
			platforms = network
		default:
			platforms = handle
		}
	}
	content := str(in, "content")
	if content == "" {
		content = str(in, "message")
	}
	status := str(in, "status")
	if status == "" {
		status = "draft"
	}
	_, err := r.db(ctx).Exec(ctx, `
		INSERT INTO crm_social_posts (id, platforms, content, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NOW(),NOW())
	`, id, platforms, content, status)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func normalizeSocialAliases(row map[string]any) {
	if row == nil {
		return
	}
	platforms, _ := row["platforms"].(string)
	content, _ := row["content"].(string)
	row["network"] = platforms
	row["handle"] = platforms
	row["message"] = content
}

func (r *Repository) GetSocialPost(ctx context.Context, id string) (map[string]any, error) {
	row, err := r.GetGenericRow(ctx, "crm_social_posts", "platforms, content, status, scheduled_at", id)
	if err != nil {
		return nil, err
	}
	normalizeSocialAliases(row)
	return row, nil
}

func (r *Repository) ListSEOKeywords(ctx context.Context, opts ListOpts) ([]map[string]any, int, error) {
	return r.listGeneric(ctx, "crm_seo_keywords", opts, "term, intent, locale, landing_page, rank")
}

func (r *Repository) CreateSEOKeyword(ctx context.Context, in map[string]any) (map[string]any, error) {
	id, _ := r.NextID(ctx, "KW", 100)
	_, err := r.db(ctx).Exec(ctx, `
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
	_, err := r.db(ctx).Exec(ctx, `
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
	row := r.db(ctx).QueryRow(ctx, `SELECT colors, fonts, voice, updated_at FROM crm_brand_kit WHERE id = 'default'`)
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
	var campaigns, mqls, assets, events int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_campaigns WHERE status = 'live'`).Scan(&campaigns)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_mqls`).Scan(&mqls)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_content_assets`).Scan(&assets)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_events WHERE status IN ('planned','live')`).Scan(&events)

	var plannedBudget, campaignSpend, eventSpend float64
	_ = r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(
			COALESCE((channels->>'email')::numeric, 0) +
			COALESCE((channels->>'events')::numeric, 0) +
			COALESCE((channels->>'social')::numeric, 0) +
			COALESCE((channels->>'web')::numeric, 0)
		), 0)
		FROM crm_budget_plans
	`).Scan(&plannedBudget)
	_ = r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(budget_usd), 0) FROM crm_campaigns
		WHERE status = 'live' AND budget_usd IS NOT NULL
	`).Scan(&campaignSpend)
	_ = r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(budget_usd), 0) FROM crm_events
		WHERE status IN ('planned','live') AND budget_usd IS NOT NULL
	`).Scan(&eventSpend)

	return map[string]any{
		"campaigns_live": campaigns, "mqls_this_month": mqls, "content_assets": assets,
		"budget_burn_pct": budgetBurnPct(plannedBudget, campaignSpend+eventSpend),
		"upcoming_events": events,
	}, nil
}

func (r *Repository) DemandGenMetrics(ctx context.Context) (map[string]any, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT
			CASE
				WHEN LOWER(source) LIKE '%email%' THEN 'Email'
				WHEN LOWER(source) LIKE '%trade%' OR LOWER(source) LIKE '%show%' OR LOWER(source) LIKE '%event%' THEN 'Trade shows'
				WHEN LOWER(source) LIKE '%linkedin%' OR LOWER(source) LIKE '%social%' THEN 'LinkedIn'
				WHEN LOWER(source) LIKE '%web%' OR LOWER(source) LIKE '%seo%' THEN 'Web'
				ELSE 'Other'
			END AS channel,
			COUNT(*)::int AS mqls
		FROM crm_mqls
		GROUP BY 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var ch string
		var mqls int
		if err := rows.Scan(&ch, &mqls); err != nil {
			return nil, err
		}
		counts[ch] = mqls
	}
	var emailSends, socialPosts, events int
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_email_sends WHERE status IN ('sent','queued')`).Scan(&emailSends)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_social_posts WHERE LOWER(platforms) LIKE '%linkedin%'`).Scan(&socialPosts)
	_ = r.db(ctx).QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_events WHERE event_type ILIKE '%trade%'`).Scan(&events)

	var budgetSpend float64
	_ = r.db(ctx).QueryRow(ctx, `
		SELECT COALESCE(SUM(
			COALESCE((channels->>'email')::numeric, 0) +
			COALESCE((channels->>'events')::numeric, 0) +
			COALESCE((channels->>'social')::numeric, 0)
		), 0)
		FROM crm_budget_plans
	`).Scan(&budgetSpend)

	channels := []map[string]any{}
	add := func(name string, mqls int, weight float64) {
		if mqls <= 0 {
			return
		}
		cpl := 0.0
		if budgetSpend > 0 && weight > 0 {
			cpl = (budgetSpend * weight) / float64(mqls)
		}
		channels = append(channels, map[string]any{"name": name, "cpl": int(cpl), "mqls": mqls})
	}
	emailMQLs := counts["Email"]
	if emailMQLs == 0 && emailSends > 0 {
		emailMQLs = emailSends
	}
	add("Email", emailMQLs, 0.35)
	tradeMQLs := counts["Trade shows"]
	if tradeMQLs == 0 && events > 0 {
		tradeMQLs = events
	}
	add("Trade shows", tradeMQLs, 0.4)
	linkedInMQLs := counts["LinkedIn"]
	if linkedInMQLs == 0 && socialPosts > 0 {
		linkedInMQLs = socialPosts
	}
	add("LinkedIn", linkedInMQLs, 0.15)
	add("Web", counts["Web"], 0.1)
	if len(channels) == 0 {
		channels = []map[string]any{{"name": "Email", "cpl": 0, "mqls": 0}}
	}
	return map[string]any{"channels": channels, "email_sends": emailSends, "social_posts": socialPosts, "events": events}, nil
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
	if err := r.db(ctx).QueryRow(ctx, "SELECT COUNT(*)::int FROM "+table).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`
		SELECT row_to_json(x)::text FROM (
			SELECT id, %s, created_at, updated_at FROM %s ORDER BY created_at DESC LIMIT $1 OFFSET $2
		) x`, cols, table)
	rows, err := r.db(ctx).Query(ctx, q, opts.Limit, opts.Offset)
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
