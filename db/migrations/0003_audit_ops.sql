BEGIN;

CREATE TABLE IF NOT EXISTS crm_audit_entries (
    id           TEXT PRIMARY KEY,
    logged_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_name    TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL,
    detail       TEXT NOT NULL DEFAULT '',
    entity_type  TEXT,
    entity_id    TEXT
);

CREATE INDEX IF NOT EXISTS crm_audit_entries_logged_at_idx ON crm_audit_entries (logged_at DESC);

CREATE TABLE IF NOT EXISTS crm_api_audit (
    id           BIGSERIAL PRIMARY KEY,
    logged_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    method       TEXT NOT NULL,
    path         TEXT NOT NULL,
    status_code  INT NOT NULL DEFAULT 0,
    user_name    TEXT NOT NULL DEFAULT '',
    duration_ms  INT NOT NULL DEFAULT 0,
    client_ip    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS crm_api_audit_logged_at_idx ON crm_api_audit (logged_at DESC);

CREATE TABLE IF NOT EXISTS crm_loyalty_tier_rules (
    id                  TEXT PRIMARY KEY DEFAULT 'default',
    name                TEXT NOT NULL DEFAULT 'Pearl Club v2 · 2026',
    bronze_min_orders   INT NOT NULL DEFAULT 2,
    bronze_min_revenue  NUMERIC(18,2) NOT NULL DEFAULT 2,
    silver_min_orders   INT NOT NULL DEFAULT 4,
    silver_min_revenue  NUMERIC(18,2) NOT NULL DEFAULT 6,
    gold_min_orders     INT NOT NULL DEFAULT 8,
    gold_min_revenue    NUMERIC(18,2) NOT NULL DEFAULT 14,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by          TEXT NOT NULL DEFAULT ''
);

INSERT INTO crm_loyalty_tier_rules (id) VALUES ('default')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS crm_buying_signals (
    id           TEXT PRIMARY KEY,
    account_id   TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name TEXT NOT NULL DEFAULT '',
    signal_type  TEXT NOT NULL DEFAULT '',
    strength     TEXT NOT NULL DEFAULT 'medium',
    action_hint  TEXT NOT NULL DEFAULT '',
    observed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_buying_signals_observed_idx ON crm_buying_signals (observed_at DESC);

COMMIT;
