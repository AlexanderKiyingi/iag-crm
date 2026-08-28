package store

import (
	"context"

	"github.com/iag/crm/backend/internal/models"
)

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
	return r.LiveBridgeStatus(ctx)
}
