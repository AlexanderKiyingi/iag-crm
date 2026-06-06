// Package bridge syncs DMS outlets into CRM bridge tables.
package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/iag/crm/backend/internal/dmsclient"
	"github.com/iag/crm/backend/internal/store"
)

// Service coordinates DMS → CRM outlet sync.
type Service struct {
	Repo *store.Repository
	DMS  *dmsclient.Client
}

// Result summarizes a sync run.
type Result struct {
	SyncedAt        time.Time `json:"synced_at"`
	OutletsFetched  int       `json:"outlets_fetched"`
	OutletsUpserted int       `json:"outlets_upserted"`
	PendingCreated  int       `json:"pending_created"`
	Events          int       `json:"events"`
	Status          string    `json:"status"`
}

// Sync pulls outlets from DMS and updates CRM bridge state.
func (s *Service) Sync(ctx context.Context) (Result, error) {
	res := Result{SyncedAt: time.Now().UTC(), Status: "connected"}
	if s == nil || s.Repo == nil {
		return res, fmt.Errorf("bridge service not configured")
	}
	if s.DMS == nil || !s.DMS.Enabled() {
		res.Status = "dms_unconfigured"
		_ = s.Repo.AppendBridgeSyncLog(ctx, "DMS client not configured — sync skipped")
		return res, nil
	}

	offset := 0
	limit := 100
	for {
		outlets, total, err := s.DMS.ListOutlets(ctx, limit, offset)
		if err != nil {
			_ = s.Repo.AppendBridgeSyncLog(ctx, "DMS sync failed: "+err.Error())
			return res, err
		}
		res.OutletsFetched += len(outlets)
		for _, o := range outlets {
			upserted, pending, err := s.Repo.UpsertOutletFromDMS(ctx, o)
			if err != nil {
				slog.Warn("outlet upsert", "id", o.ID, "err", err)
				continue
			}
			if upserted {
				res.OutletsUpserted++
			}
			if pending {
				res.PendingCreated++
			}
		}
		offset += len(outlets)
		if offset >= total || len(outlets) == 0 {
			break
		}
	}
	res.Events = res.OutletsUpserted + res.PendingCreated
	_ = s.Repo.TouchBridgeStreams(ctx)
	_ = s.Repo.AppendBridgeSyncLog(ctx, fmt.Sprintf(
		"DMS sync complete: fetched=%d upserted=%d pending=%d",
		res.OutletsFetched, res.OutletsUpserted, res.PendingCreated,
	))
	return res, nil
}
