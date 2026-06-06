// Package consumer subscribes to iag.commercial and applies upstream events to CRM.
package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	platformevents "github.com/alvor-technologies/iag-platform-go/events"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/store"
)

const (
	TypeContractCreated       = "contracts.contract.created"
	TypeContractStatusChanged = "contracts.contract.status_changed"
	TypeOutletCreated         = "dms.outlet.created"
	TypeOrgMemberAdded        = "users.org.member_added"
	TypeOrgMemberRemoved      = "users.org.member_removed"
)

// Handler routes envelopes to per-type handlers.
type Handler struct {
	Repo *store.Repository
}

// Handle implements platformevents.Handler.
func (h *Handler) Handle(ctx context.Context, env platformevents.Envelope) error {
	if env.Source == events.Source {
		return nil
	}
	switch env.Type {
	case TypeContractCreated:
		return h.handleContractCreated(ctx, env)
	case TypeContractStatusChanged:
		return h.handleContractStatusChanged(ctx, env)
	case TypeOutletCreated:
		return h.handleOutletCreated(ctx, env)
	case TypeOrgMemberAdded, TypeOrgMemberRemoved:
		return h.handleOrgMember(ctx, env)
	default:
		return nil
	}
}

func (h *Handler) handleContractCreated(ctx context.Context, env platformevents.Envelope) error {
	quoteID := stringField(env.Data, "crmQuoteId")
	if quoteID == "" {
		quoteID = stringField(env.Data, "quoteId")
	}
	contractNo := stringField(env.Data, "no")
	if contractNo == "" {
		contractNo = stringField(env.Data, "contractNo")
	}
	if quoteID == "" || contractNo == "" {
		return nil
	}
	if err := h.Repo.SetQuoteContractRef(ctx, quoteID, contractNo); err != nil {
		slog.Warn("contract link quote", "quote", quoteID, "contract", contractNo, "err", err)
	}
	return nil
}

func (h *Handler) handleContractStatusChanged(ctx context.Context, env platformevents.Envelope) error {
	quoteID := stringField(env.Data, "crmQuoteId")
	contractNo := stringField(env.Data, "no")
	status := stringField(env.Data, "status")
	if quoteID == "" || contractNo == "" {
		return nil
	}
	_ = h.Repo.SetQuoteContractRef(ctx, quoteID, contractNo)
	if status != "" {
		_ = h.Repo.AppendBridgeSyncLog(ctx, fmt.Sprintf("Contract %s status → %s (quote %s)", contractNo, status, quoteID))
	}
	return nil
}

func (h *Handler) handleOutletCreated(ctx context.Context, env platformevents.Envelope) error {
	outletID := stringField(env.Data, "id")
	name := stringField(env.Data, "name")
	if outletID == "" {
		return platformevents.Permanent(fmt.Errorf("dms.outlet.created missing id"))
	}
	created, err := h.Repo.EnsurePendingImport(ctx, outletID, name)
	if err != nil {
		return err
	}
	if created {
		_ = h.Repo.AppendBridgeSyncLog(ctx, "Pending import from DMS outlet "+outletID)
	}
	return nil
}

func (h *Handler) handleOrgMember(ctx context.Context, env platformevents.Envelope) error {
	orgID := stringField(env.Data, "orgId")
	userEmail := stringField(env.Data, "email")
	if orgID == "" || userEmail == "" {
		return nil
	}
	_ = h.Repo.AppendBridgeSyncLog(ctx, fmt.Sprintf("Org %s member event %s for %s", orgID, env.Type, userEmail))
	return nil
}

func stringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
