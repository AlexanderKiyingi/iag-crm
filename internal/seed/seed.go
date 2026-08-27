package seed

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run is intentionally a no-op.
//
// It used to load the CRM prototype dataset — 18 accounts, 15 contacts, 10 deals and
// the marketing bridge behind them — into an empty database. Migration
// 0009_purge_demo_seed.sql removed every one of those rows from every environment, and
// config.seedOnEmpty now refuses to seed in production at all. A CRM database starts
// empty and fills up with what operators actually enter.
//
// The function and its call site in main.go are kept so a legitimate seed (reference
// data, a customer import) has an obvious place to live.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	slog.Info("CRM seed skipped — the demo dataset was purged and is not reloaded")
	return nil
}
