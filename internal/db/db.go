package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	platformdb "github.com/alvor-technologies/iag-platform-go/db"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return nil, errors.New("DATABASE_URL is empty")
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// Sizing comes from the shared platform package so every service is tuned
	// through the same variables. The previous MaxConns default of 50 is kept
	// deliberately rather than dropping to the package default of 10: cutting a
	// busy service's pool fivefold is a capacity decision to make on measured
	// evidence, not a side effect of a refactor.
	pcfg := platformdb.ConfigFromEnv("crm, public")
	pcfg.URL = url
	if pcfg.MaxConns == 0 {
		pcfg.MaxConns = 50
	}
	cfg, err = platformdb.BuildPoolConfig(pcfg)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// Resolve unqualified names to this service's own schema first, falling back
	// to public so legacy tables that still live there keep resolving. On the
	// shared Railway database this isolates crm from the global public namespace
	// and its single global schema_migrations ledger. See internal/migrate.

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func intEnv(key string, fallback int32) int32 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return int32(n)
}
