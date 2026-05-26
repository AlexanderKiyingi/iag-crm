package store

import (
	"context"

	"github.com/iag/crm/backend/internal/models"
)

var rangeMetrics = map[string]models.RangeMetrics{
	"today": {
		Label: "Today", Pipeline: "$184K", PipelineDelta: "▲ 4.8%", Outlets: "412",
		WinRate: "31.2", Cycle: "42", NRR: "108",
		Series: []int{180, 176, 170, 164, 160, 156, 150, 144, 138, 132, 128, 122},
		XLabels: []string{"08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "now"},
	},
	"week": {
		Label: "Week", Pipeline: "$1.84M", PipelineDelta: "▲ 18.2%", Outlets: "2,162",
		WinRate: "34.6", Cycle: "38", NRR: "112",
		Series: []int{180, 168, 174, 152, 158, 132, 124, 108, 96, 84, 76, 62},
		XLabels: []string{"w-12", "w-9", "w-6", "w-3", "w-1", "now"},
	},
	"month": {
		Label: "Month", Pipeline: "$6.42M", PipelineDelta: "▲ 24.4%", Outlets: "2,481",
		WinRate: "36.8", Cycle: "34", NRR: "114",
		Series: []int{200, 180, 170, 148, 140, 120, 108, 98, 84, 68, 58, 48},
		XLabels: []string{"m-12", "m-9", "m-6", "m-3", "m-1", "now"},
	},
	"quarter": {
		Label: "Quarter", Pipeline: "$18.4M", PipelineDelta: "▲ 31.7%", Outlets: "2,847",
		WinRate: "38.2", Cycle: "31", NRR: "118",
		Series: []int{220, 210, 196, 182, 170, 150, 134, 120, 98, 80, 68, 52},
		XLabels: []string{"q-4", "q-3", "q-2", "q-1", "q-0", "now"},
	},
}

func OverviewMetrics(rangeKey string) models.RangeMetrics {
	if m, ok := rangeMetrics[rangeKey]; ok {
		return m
	}
	return rangeMetrics["week"]
}

func Notifications() []models.Notification {
	return []models.Notification{
		{Kind: "crm", Icon: "$", Title: "Deal advanced · Matsuri Coffee", Body: "$280K · Bugisu AA · Proposal → Negotiation", Time: "3m", Go: "deals"},
		{Kind: "dms", Icon: "⇆", Title: "DMS sync · 14 new outlets", Body: "SAFARI · auto-imported as accounts · awaiting owner", Time: "12m", Go: "bridge"},
		{Kind: "err", Icon: "!", Title: "Ticket TKT-218 escalated", Body: "Kaffee Haus · moisture spec breach · 2h SLA left", Time: "28m", Go: "tickets"},
		{Kind: "info", Icon: "+", Title: "3 hot leads from website", Body: "Score ≥ 80 · auto-routed to B. Kato", Time: "1h", Go: "leads"},
		{Kind: "ok", Icon: "✓", Title: "Campaign live · Q2 Origin Tour", Body: "Open rate 47.2% · 18 replies in 4h", Time: "2h", Go: "campaigns"},
		{Kind: "warn", Icon: "⏰", Title: "Renewal due · Union Hand-Roasted", Body: "Annual contract · 14 days · last touch 47d ago", Time: "3h", Go: "accounts"},
		{Kind: "crm", Icon: "📄", Title: "Quote QTE-0421 viewed 4×", Body: "Seoul Bean Lab · $88K · 6 lots · sent 2d ago", Time: "4h", Go: "quotes"},
		{Kind: "warn", Icon: "★", Title: "Tier upgrade · 2 outlets", Body: "OUT-02331, OUT-01892 → SAFARI Gold", Time: "5h", Go: "loyalty"},
		{Kind: "info", Icon: "🧠", Title: "AI: Churn risk on 6 accounts", Body: "NRR exposure $182K · review weekly digest", Time: "6h", Go: "insights"},
	}
}

func BootstrapPayload() models.Bootstrap {
	return models.Bootstrap{
		Service:   "crm",
		Role:      "md",
		RoleLabel: "Managing Director",
		Pages:     []string{"*"},
		Modals:    []string{"*"},
		PageTitles: map[string]string{
			"overview": "Customer Tower", "pipeline": "Sales Pipeline", "accounts": "Account Directory",
			"contacts": "Contact Directory", "leads": "Leads", "deals": "Deals & Forecast",
			"quotes": "Quotes & Contracts", "activities": "Activities & Tasks", "campaigns": "Campaigns",
		},
		SyncStatus: "connected",
	}
}

func (r *Repository) BridgeStatus(ctx context.Context) map[string]any {
	var bridged int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM crm_accounts WHERE dms_bridged = TRUE`).Scan(&bridged)
	return map[string]any{
		"connected":       true,
		"streams":         6,
		"accounts_bridged": bridged,
		"last_sync_at":    "just now",
		"pending_imports": 14,
	}
}
