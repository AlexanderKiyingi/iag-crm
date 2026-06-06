package seed

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iag/crm/backend/internal/store"
)

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	repo := store.New(pool)
	empty, err := repo.IsEmpty(ctx)
	if err != nil {
		return err
	}
	if !empty {
		slog.Info("seed skipped — data already present")
		return nil
	}
	slog.Info("seeding CRM demo data from index.html prototype")

	now := time.Now().UTC()
	touch := func(hoursAgo int) time.Time {
		return now.Add(-time.Duration(hoursAgo) * time.Hour)
	}

	accounts := []struct {
		id, name, typ, country, segment, owner, value string
		health                                         int
		status                                         string
		bridged                                        bool
		hoursAgo                                       int
	}{
		{"ACC-0421", "Matsuri Coffee Japan", "Export", "JP", "Roaster", "Bernard Kato", "USD $1.2M", 92, "active", false, 2},
		{"ACC-0418", "Kaffee Haus Hamburg", "Export", "DE", "Roaster", "Elizabeth Hoek", "EUR €840K", 88, "active", false, 5},
		{"ACC-0415", "Amsterdam Specialty", "Export", "NL", "Roaster", "Elizabeth Hoek", "USD $1.4M", 91, "active", false, 24},
		{"ACC-0412", "Union Hand-Roasted UK", "Export", "GB", "Roaster", "Elizabeth Hoek", "GBP £380K", 85, "active", false, 48},
		{"ACC-0409", "Seoul Bean Lab", "Export", "KR", "Roaster", "Bernard Kato", "USD $295K", 78, "nurture", false, 72},
		{"ACC-0406", "Emirates Coffee UAE", "Export", "AE", "Distributor", "Bernard Kato", "USD $640K", 82, "active", false, 96},
		{"ACC-0403", "Nordic Roasters Sweden", "Export", "SE", "Roaster", "Elizabeth Hoek", "EUR €420K", 80, "active", false, 144},
		{"ACC-0398", "Capital Shoppers", "Domestic", "UG", "Retail", "Aisha Achieng", "UGX 312M", 94, "active", true, 1},
		{"ACC-0395", "Cafe Javas", "Domestic", "UG", "HoReCa", "Aisha Achieng", "UGX 248M", 89, "active", true, 3},
		{"ACC-0392", "Lugogo Hyper", "Domestic", "UG", "Retail", "Aisha Achieng", "UGX 196M", 86, "active", true, 4},
		{"ACC-0389", "Nakawa Mart", "Domestic", "UG", "Retail", "Aisha Achieng", "UGX 142M", 81, "active", true, 7},
		{"ACC-0386", "Mengo Trading Centre", "Domestic", "UG", "Wholesale", "Aisha Achieng", "UGX 98M", 74, "nurture", true, 24},
		{"ACC-0383", "Endiro Coffee", "Domestic", "UG", "HoReCa", "Joseph Mwesigwa", "UGX 88M", 83, "active", false, 24},
		{"ACC-0380", "Gulu Coffee Hub", "Domestic", "UG", "Wholesale", "David Anyama", "UGX 64M", 71, "active", false, 48},
		{"ACC-0377", "Arua Distributors", "Domestic", "UG", "Wholesale", "David Anyama", "UGX 52M", 68, "nurture", false, 72},
		{"ACC-0374", "Java House Kampala", "Domestic", "UG", "HoReCa", "Joseph Mwesigwa", "UGX 124M", 87, "active", false, 96},
		{"ACC-0371", "Acacia Mall Outlets", "Domestic", "UG", "Retail", "Aisha Achieng", "UGX 76M", 79, "active", true, 120},
		{"ACC-0368", "Mbarara Coffee Centre", "Domestic", "UG", "Wholesale", "David Anyama", "UGX 42M", 65, "risk", false, 192},
	}
	for _, a := range accounts {
		lt := touch(a.hoursAgo)
		_, err := pool.Exec(ctx, `
			INSERT INTO crm_accounts (id, name, account_type, country, segment, owner, value_display, health_score, status, dms_bridged, last_touch_at, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11,$11)
		`, a.id, a.name, a.typ, a.country, a.segment, a.owner, a.value, a.health, a.status, a.bridged, lt)
		if err != nil {
			return err
		}
		_, _ = pool.Exec(ctx, `INSERT INTO crm_id_counters (prefix, next_value) VALUES ('ACC', 500) ON CONFLICT DO NOTHING`)
	}

	contacts := []struct {
		id, name, title, account, email, phone, owner string
		primary                                         bool
	}{
		{"CON-1142", "Yuki Tanaka", "Procurement Director", "Matsuri Coffee Japan", "y.tanaka@matsuri.jp", "+81 3 5421 8800", "Bernard Kato", true},
		{"CON-1138", "Klaus Bergmann", "Head of Sourcing", "Kaffee Haus Hamburg", "klaus@kaffeehaus.de", "+49 40 8821 0142", "Elizabeth Hoek", true},
		{"CON-1135", "Sanne van der Berg", "Green Coffee Buyer", "Amsterdam Specialty", "sanne@amsterdamspecialty.nl", "+31 20 612 8800", "Elizabeth Hoek", true},
		{"CON-1132", "James Hoffmann", "Director of Coffee", "Union Hand-Roasted UK", "james@unionroasted.com", "+44 20 7253 4000", "Elizabeth Hoek", true},
		{"CON-1129", "Min-jun Park", "Founder & Roaster", "Seoul Bean Lab", "minjun@seoulbeanlab.kr", "+82 2 6204 5500", "Bernard Kato", true},
		{"CON-1126", "Ahmed Al-Rashid", "Procurement Manager", "Emirates Coffee UAE", "ahmed@emiratescoffee.ae", "+971 4 354 8800", "Bernard Kato", true},
		{"CON-1123", "Erik Lindqvist", "Coffee Director", "Nordic Roasters Sweden", "erik@nordicroasters.se", "+46 8 6121 4400", "Elizabeth Hoek", true},
		{"CON-1120", "Sarah Nakato", "Category Manager", "Capital Shoppers", "snakato@capitalshoppers.ug", "+256 414 540 100", "Aisha Achieng", true},
		{"CON-1117", "Mandela Auma", "F&B Director", "Cafe Javas", "mandela@cafejavas.co.ug", "+256 414 250 800", "Aisha Achieng", true},
		{"CON-1114", "Patrick Mukasa", "Procurement Officer", "Lugogo Hyper", "patrick@lugogohyper.ug", "+256 414 555 220", "Aisha Achieng", true},
		{"CON-1111", "Grace Nansubuga", "Buyer", "Nakawa Mart", "grace@nakawamart.ug", "+256 414 222 008", "Aisha Achieng", false},
		{"CON-1108", "Robert Ssebunya", "General Manager", "Mengo Trading Centre", "robert@mengotc.ug", "+256 414 270 660", "Aisha Achieng", true},
		{"CON-1105", "Cathy Adong", "Outlet Manager", "Endiro Coffee", "cathy@endiro.com", "+256 414 533 220", "Joseph Mwesigwa", true},
		{"CON-1102", "Geoffrey Onen", "Director", "Gulu Coffee Hub", "geoffrey@gulucoffee.ug", "+256 471 432 100", "David Anyama", true},
		{"CON-1099", "Maureen Akumu", "Operations Lead", "Java House Kampala", "maureen@javahouse.ug", "+256 414 600 800", "Joseph Mwesigwa", true},
	}
	for _, c := range contacts {
		var accountID string
		_ = pool.QueryRow(ctx, `SELECT id FROM crm_accounts WHERE name = $1`, c.account).Scan(&accountID)
		_, err := pool.Exec(ctx, `
			INSERT INTO crm_contacts (id, account_id, account_name, name, title, email, phone, owner, is_primary, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		`, c.id, nullIfEmpty(accountID), c.account, c.name, c.title, c.email, c.phone, c.owner, c.primary)
		if err != nil {
			return err
		}
	}

	deals := []struct {
		id, name, account, stage, owner, currency, display, desc, source string
		amount, prob                                                        float64
		dms                                                                 bool
		daysAgo                                                             int
	}{
		{"DEAL-0431", "Toronto Coffee Lab", "Toronto Coffee Lab", "lead", "Bernard Kato", "USD", "$28K", "Bugisu AA · 8 bags sample req.", "Inbound · website", 28000, 18, false, 3},
		{"DEAL-0429", "Coffea Roasters Berlin", "Coffea Roasters Berlin", "lead", "Elizabeth Hoek", "USD", "$84K", "Bugisu AB · annual contract", "Trade show · Anuga", 84000, 18, false, 5},
		{"DEAL-0426", "Capital Shoppers Bukoto", "Capital Shoppers", "lead", "Aisha Achieng", "UGX", "UGX 18M", "HARAKA Instant · listing", "DMS · auto-import", 18000000, 18, true, 9},
		{"DEAL-0422", "Capital Shoppers chain", "Capital Shoppers", "qualified", "Aisha Achieng", "UGX", "UGX 96M", "8 outlets · HARAKA + Bugisu", "Demo · Wed", 96000000, 38, true, 4},
		{"DEAL-0420", "Tokyo Origin Coffee", "Tokyo Origin Coffee", "qualified", "Bernard Kato", "USD", "$96K", "Bugisu AA + Reserve", "Sample sent · 4d", 96000, 38, false, 11},
		{"DEAL-0418", "Kaffee Haus Bergmann", "Kaffee Haus Hamburg", "proposal", "Bernard Kato", "USD", "$195K", "Annual · +12% volume · Q3 ship", "Sent · 3d", 195000, 58, false, 3},
		{"DEAL-0419", "Seoul Bean Lab", "Seoul Bean Lab", "proposal", "Joseph Mwesigwa", "USD", "$88K", "6 lots · honey + natural · 24mo", "Viewed 4×", 88000, 58, false, 2},
		{"DEAL-0421", "Matsuri Q3 expansion", "Matsuri Coffee Japan", "negotiation", "Bernard Kato", "USD", "$280K", "3 lots Bugisu AA · Yokohama port", "Call · Tue 9am", 280000, 78, false, 1},
		{"DEAL-0415", "Amsterdam dual-quarter", "Amsterdam Specialty", "negotiation", "Elizabeth Hoek", "USD", "$340K", "All arabica + Sipi natural · 24mo", "Verbal yes", 340000, 88, false, 2},
		{"DEAL-0411", "Union seasonal renewal", "Union Hand-Roasted UK", "won", "Elizabeth Hoek", "USD", "$125K", "Bugisu AB seasonal", "Closed won", 125000, 100, false, 0},
	}
	for _, d := range deals {
		created := now.Add(-time.Duration(d.daysAgo) * 24 * time.Hour)
		var accountID string
		_ = pool.QueryRow(ctx, `SELECT id FROM crm_accounts WHERE name = $1 LIMIT 1`, d.account).Scan(&accountID)
		wonAt := any(nil)
		if d.stage == "won" {
			wonAt = created
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO crm_deals (id, name, account_id, account_name, stage, probability, owner, currency, amount, amount_display,
				description, source, dms_linked, created_at, updated_at, won_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14,$15)
		`, d.id, d.name, nullIfEmpty(accountID), d.account, d.stage, int(d.prob), d.owner, d.currency, d.amount, d.display,
			d.desc, d.source, d.dms, created, wonAt)
		if err != nil {
			return err
		}
	}

	leads := []struct {
		id, name, company, email, source, segment, owner, status string
		score                                                      int
	}{
		{"LEAD-0117", "Blue Bottle Korea", "Blue Bottle Korea", "buyers@bluebottlekorea.kr", "Website form", "Roaster · specialty", "Bernard Kato", "qualifying", 82},
		{"LEAD-0114", "Toronto Coffee Lab", "Toronto Coffee Lab", "procurement@torontocoffee.ca", "Trade show", "Roaster · specialty", "Bernard Kato", "hot", 88},
		{"LEAD-0111", "Helsinki Roastery", "Helsinki Roastery", "hello@helsinkiroastery.fi", "Referral", "Roaster · specialty", "Elizabeth Hoek", "warm", 74},
	}
	for _, l := range leads {
		_, err := pool.Exec(ctx, `
			INSERT INTO crm_leads (id, name, company, email, source, segment, score, status, owner, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		`, l.id, l.name, l.company, l.email, l.source, l.segment, l.score, l.status, l.owner)
		if err != nil {
			return err
		}
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO crm_quotes (id, ref, account_id, account_name, deal_id, template, currency, incoterms, payment_terms, total, status, owner, line_items, created_at, updated_at)
		VALUES
		('QTE-0421','QTE-0421','ACC-0409','Seoul Bean Lab','DEAL-0419','FOB Mombasa · standard','USD','FOB Mombasa','L/C at sight',88000,'sent','Joseph Mwesigwa','[{"lot_ref":"LIMS-SP-04418","quantity":320}]',NOW(),NOW()),
		('QTE-0418','QTE-0418','ACC-0418','Kaffee Haus Hamburg','DEAL-0418','CIF Hamburg','EUR','CIF Hamburg','L/C 60-days',195000,'draft','Bernard Kato','[]',NOW(),NOW())
	`)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO crm_tickets (id, account_id, account_name, subject, ticket_type, priority, channel, status, owner, description, sla_due_at, created_at, updated_at)
		VALUES
		('TKT-0218','ACC-0418','Kaffee Haus Hamburg','Moisture spec breach · lot Q2-018','Quality · moisture / spec','P1','Email','escalated','Bernard Kato','Cupping score below contract minimum.',NOW() + INTERVAL '2 hours',NOW(),NOW()),
		('TKT-0094','ACC-0398','Capital Shoppers','Packaging tear · Ntinda branch','Logistics · damage','P2','DMS bridge auto-create','open','Aisha Achieng','Shelf-ready unit damaged in transit.',NOW() + INTERVAL '24 hours',NOW(),NOW())
	`)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO crm_campaigns (id, name, campaign_type, audience, budget_usd, owner, goal, status, created_at, updated_at)
		VALUES
		('CMP-0042','Q2 Origin Tour','Email · sequence','Specialty roasters · EU/APAC',14000,'Head of Marketing','80 MQLs · 8 SQLs','live',NOW(),NOW()),
		('CMP-0038','HARAKA Instant Trade Promo','Trade · BOGO','HoReCa · Kampala',8000,'Aisha Achieng','UGX 96M influenced pipeline','draft',NOW(),NOW())
	`)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO crm_activities (id, activity_type, subject, body, account_id, account_name, deal_id, owner, occurred_at, created_at)
		VALUES
		('ACT-1001','Call · outbound','Tokyo · final pricing call · 35m','Discussed Q3 volume and port routing.','ACC-0421','Matsuri Coffee Japan','DEAL-0421','Bernard Kato',NOW() - INTERVAL '2 hours',NOW()),
		('ACT-1002','Email · inbound','Quote QTE-0421 viewed','Seoul Bean Lab opened quote 4 times.','ACC-0409','Seoul Bean Lab','DEAL-0419','Joseph Mwesigwa',NOW() - INTERVAL '4 hours',NOW())
	`)
	if err != nil {
		return err
	}

	// Advance ID counters past seeded IDs.
	counters := map[string]int64{
		"ACC": 500, "CON": 1200, "DEAL": 500, "LEAD": 200, "QTE": 500, "TKT": 300, "CMP": 100, "ACT": 1000,
	}
	for prefix, n := range counters {
		_, _ = pool.Exec(ctx, `
			INSERT INTO crm_id_counters (prefix, next_value) VALUES ($1, $2)
			ON CONFLICT (prefix) DO UPDATE SET next_value = GREATEST(crm_id_counters.next_value, EXCLUDED.next_value)
		`, prefix, n)
	}

	slog.Info("CRM seed complete", "accounts", len(accounts), "contacts", len(contacts), "deals", len(deals))
	if err := seedMarketingBridge(ctx, pool); err != nil {
		return err
	}
	return nil
}

func seedMarketingBridge(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_segments`).Scan(&n); err != nil || n > 0 {
		return err
	}

	streams := []struct{ id, name, dir string }{
		{"STR-01", "DMS Outlets → CRM Accounts", "inbound"},
		{"STR-02", "DMS Orders → CRM Deals", "inbound"},
		{"STR-03", "DMS Distributors → CRM Accounts", "inbound"},
		{"STR-04", "DMS Tickets → CRM Claims", "inbound"},
		{"STR-05", "DMS Loyalty → Outlet KPI", "inbound"},
		{"STR-06", "DMS Beats → CRM Activities", "inbound"},
	}
	for _, s := range streams {
		_, _ = pool.Exec(ctx, `
			INSERT INTO crm_bridge_streams (id, name, direction, status, last_sync_at, record_count)
			VALUES ($1,$2,$3,'connected',NOW(),14)
		`, s.id, s.name, s.dir)
	}

	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_segments (id, name, kind, refresh, rules, member_count, created_at, updated_at) VALUES
		('SEG-001','Specialty roasters · EU/APAC','dynamic','5 min','type = roaster AND region IN (EU, APAC)',842,NOW(),NOW()),
		('SEG-002','HoReCa · Kampala','dynamic','5 min','segment = HoReCa AND country = UG',186,NOW(),NOW()),
		('SEG-003','Pearl Club · Gold','static','1 day','loyalty_tier = Gold',124,NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_journeys (id, name, trigger, template, goal, status, enrolled, conversion, created_at, updated_at) VALUES
		('JRN-001','Welcome → first cupping','CRM · new contact','Welcome → first cupping','Sample request in 30d','active',428,18.4,NOW(),NOW()),
		('JRN-002','HoReCa onboarding','DMS · new outlet','HoReCa onboarding','First order in 14d','active',96,24.1,NOW(),NOW())
	`)
	if err := EnsureJourneySteps(ctx, pool); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_email_sends (id, subject, template, body, status, created_at, updated_at) VALUES
		('EML-001','Origin story Q2','Welcome','Body','sent',NOW(),NOW()),
		('EML-002','Sample follow-up','Follow-up','Body','queued',NOW(),NOW())
		ON CONFLICT (id) DO NOTHING
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_social_posts (id, platforms, content, status, created_at, updated_at) VALUES
		('SOC-001','LinkedIn','Q2 origin tour','draft',NOW(),NOW())
		ON CONFLICT (id) DO NOTHING
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_budget_plans (id, name, quarter, owner, channels, mql_target, sql_target, won_target, created_at, updated_at) VALUES
		('BDG-001','Q2 demand gen','Q2','marketing','{"email":12000,"events":18000,"social":6000}',200,80,500000,NOW(),NOW())
		ON CONFLICT (id) DO NOTHING
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_personas (id, name, buyer_role, region, seniority, content_tags, story, created_at, updated_at) VALUES
		('PSN-001','Yuki · Tokyo specialty buyer','Decision-maker','APAC','Director','specialty,micro-lot','Mid-30s Q-grader sourcing for 12-shop chain.',NOW(),NOW()),
		('PSN-002','Sarah · Kampala category manager','Influencer','Domestic · Uganda','Manager','retail,HoReCa','Modern trade buyer focused on margin and velocity.',NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_events (id, name, event_type, city, budget_usd, mql_target, registrations, status, created_at, updated_at) VALUES
		('EVT-001','Specialty Coffee Expo · Boston','Trade show','Boston',28000,80,142,'planned',NOW(),NOW()),
		('EVT-002','Mt. Elgon · buyer immersion','Origin tour','Mbale',18000,24,18,'planned',NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_mqls (id, name, company, score, source, owner, status, created_at, updated_at) VALUES
		('MQL-001','Ji-ho Park','Seoul Bean Lab',94,'Email campaign','Bernard Kato','accepted',NOW(),NOW()),
		('MQL-002','Eva Hoek','Amsterdam Specialty',91,'Trade show','Elizabeth Hoek','new',NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_loyalty_tiers (id, name, min_points, benefits, multiplier) VALUES
		('TIER-BRONZE','Bronze',0,'Base pricing',1.0),
		('TIER-GOLD','SAFARI Gold',5000,'Priority allocation + 2% rebate',1.25),
		('TIER-PLAT','Pearl Platinum',12000,'Dedicated rep + co-brand support',1.5)
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_outlets (id, dms_ref, account_id, name, city, segment, health, owner, created_at, updated_at) VALUES
		('OUT-1247','OUT-00214','ACC-0398','Capital Shoppers Ntinda','Kampala','Modern Trade',94,'Aisha Achieng',NOW(),NOW()),
		('OUT-0892','OUT-02331','ACC-0395','Cafe Javas (HQ)','Kampala','HoReCa Chain',91,'Aisha Achieng',NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_export_customers (id, name, country, currency, incoterms, credit_limit, erp_ref, created_at, updated_at) VALUES
		('EXP-001','Matsuri Coffee Japan','JP','USD','CIF Tokyo',500000,'ERP-CUS-4421',NOW(),NOW()),
		('EXP-002','Kaffee Haus Hamburg','DE','EUR','CIF Hamburg',350000,'ERP-CUS-4418',NOW(),NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_brand_kit (id, colors, fonts, voice, updated_at) VALUES
		('default','{"crm":"#7C3AED","primary":"#7C3AED"}','{"display":"Fraunces","body":"DM Sans"}','Specialty coffee · traceable · partner-first',NOW())
	`)
	_, _ = pool.Exec(ctx, `
		INSERT INTO crm_buying_signals (id, account_id, account_name, signal_type, strength, action_hint, observed_at) VALUES
		('SIG-001', NULL, 'Seoul Bean Lab', 'Quote re-opened 4×', 'high', 'Schedule follow-up call', NOW() - INTERVAL '2 hours'),
		('SIG-002', NULL, 'Capital Shoppers', 'Reorder 6 days early', 'high', 'Propose volume tier upgrade', NOW() - INTERVAL '5 hours'),
		('SIG-003', NULL, 'Matsuri Trading', 'Contract PDF downloaded', 'medium', 'Send pricing clarification', NOW() - INTERVAL '1 day')
		ON CONFLICT (id) DO NOTHING
	`)
	slog.Info("marketing/bridge seed complete")
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
