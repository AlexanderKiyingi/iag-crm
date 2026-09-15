package models

import "strings"

// RoleSpec mirrors index.html ROLES for bootstrap / Next.js route guards.
type RoleSpec struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Full   string   `json:"full"`
	Pages  any      `json:"pages"`
	Modals any      `json:"modals"`
}

var PageTitles = map[string]string{
	"overview": "Customer Tower", "pipeline": "Sales Pipeline", "accounts": "Account Directory",
	"contacts": "Contact Directory", "leads": "Leads", "deals": "Deals & Forecast",
	"quotes": "Quotes & Contracts", "activities": "Activities & Tasks", "campaigns": "Campaigns",
	"loyalty": "Outlet Loyalty · Pearl Club", "tickets": "Service Tickets",
	"bridge": "DMS ⇆ CRM Sync", "outlet360": "Outlet 360°", "export360": "Export Customers",
	"insights": "Customer Insights", "ai": "AI Sales Copilot",
	"marketingHub": "Marketing Hub", "segments": "Segments & Audiences",
	"journeys": "Journeys & Automation", "contentLib": "Content Library",
	"brandKit": "Brand Kit", "mqls": "MQL Scoring & ROI", "events": "Events & Activations",
	"demandGen": "Demand Generation", "emailStudio": "Email Studio",
	"social": "Social & PR", "webSeo": "Web & SEO", "budget": "Budget & ROI",
	"personas": "Buyer Personas",
}

var Roles = map[string]RoleSpec{
	"md": {
		ID: "md", Label: "MD · all access", Full: "Managing Director",
		Pages: "*", Modals: "*",
	},
	"head_marketing": {
		ID: "head_marketing", Label: "Head of Marketing", Full: "Head of Marketing",
		Pages: []string{
			"overview", "pipeline", "accounts", "contacts", "leads", "campaigns",
			"marketingHub", "segments", "journeys", "contentLib", "brandKit", "mqls",
			"events", "demandGen", "emailStudio", "social", "webSeo", "budget", "personas",
			"insights", "ai",
		},
		Modals: "*",
	},
	"head_commercial": {
		ID: "head_commercial", Label: "Head of Commercial", Full: "Head of Commercial",
		Pages: []string{
			"overview", "pipeline", "accounts", "contacts", "leads", "deals", "quotes", "activities",
			"campaigns", "marketingHub", "segments", "contentLib", "mqls", "events", "demandGen",
			"budget", "personas", "tickets", "bridge", "outlet360", "export360", "loyalty",
			"insights", "ai",
		},
		Modals: "*",
	},
	"sales_rep": {
		ID: "sales_rep", Label: "Sales Rep · read", Full: "Sales Representative",
		Pages: []string{
			"overview", "pipeline", "accounts", "contacts", "leads", "deals", "quotes",
			"activities", "tickets", "outlet360", "marketingHub", "contentLib",
		},
		Modals: []string{"modalNewDeal", "modalNewLead", "modalNewContact"},
	},
	"support": {
		ID: "support", Label: "Support", Full: "Customer Support",
		Pages:  []string{"overview", "accounts", "contacts", "activities", "tickets", "outlet360"},
		Modals: []string{"modalNewContact"},
	},
	"viewer": {
		ID: "viewer", Label: "Viewer · read only", Full: "Read-only Viewer",
		Pages: []string{
			"overview", "pipeline", "accounts", "contacts", "leads", "deals", "quotes",
			"activities", "tickets", "campaigns", "insights",
		},
		Modals: []string{},
	},
}

func RoleFromGroups(groups []string, isSuperuser bool) string {
	if isSuperuser {
		return "md"
	}
	for _, g := range groups {
		switch stringsToLower(g) {
		case "md", "managing_director":
			return "md"
		case "head_marketing", "head-of-marketing":
			return "head_marketing"
		case "head_commercial", "head-of-commercial":
			return "head_commercial"
		case "sales_rep", "sales-rep", "sales":
			return "sales_rep"
		// The groups iag-authentication actually seeds (see docs/IAG_CUTOVER.md
		// in the Next.js app). None of them matched the legacy names above, so
		// every platform user — administrators and viewers included — fell to
		// the default below and was owner-scoped to records carrying their own
		// email. With the app's free-text Owner field that meant they saw
		// nothing at all.
		case "crm-administrator", "crm_administrator":
			return "md"
		case "crm-sales-manager", "crm_sales_manager":
			return "head_commercial"
		case "crm-marketing", "crm_marketing":
			return "head_marketing"
		case "crm-sales-rep", "crm_sales_rep":
			return "sales_rep"
		case "crm-support", "crm_support":
			return "support"
		case "crm-viewer", "crm_viewer":
			return "viewer"
		}
	}
	// Unknown groups fail closed to the narrowest role. Permissions still gate
	// every route under strict RBAC; this only decides row-level visibility.
	return "sales_rep"
}

// ScopedRoles are the roles confined to records they own. Kept as a set so
// the two places that scope (list and single-record fetch) cannot drift.
var ScopedRoles = map[string]bool{"sales_rep": true}

// IsScopedRole reports whether a role sees only its own records.
func IsScopedRole(role string) bool { return ScopedRoles[role] }

func stringsToLower(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "_"))
}
