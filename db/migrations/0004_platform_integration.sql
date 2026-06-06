BEGIN;

CREATE TABLE IF NOT EXISTS crm_processed_events (
    event_id     TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_event_outbox (
    id              BIGSERIAL PRIMARY KEY,
    event_type      TEXT        NOT NULL,
    event_key       TEXT,
    payload         JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispatched_at   TIMESTAMPTZ,
    attempts        INT         NOT NULL DEFAULT 0,
    last_error      TEXT
);

CREATE INDEX IF NOT EXISTS crm_event_outbox_pending_idx
    ON crm_event_outbox (available_at)
    WHERE dispatched_at IS NULL;

ALTER TABLE crm_accounts
    ADD COLUMN IF NOT EXISTS billing_org_id TEXT,
    ADD COLUMN IF NOT EXISTS billing_identity_id TEXT,
    ADD COLUMN IF NOT EXISTS finance_customer_ref TEXT;

ALTER TABLE crm_deals
    ADD COLUMN IF NOT EXISTS finance_ar_ref TEXT;

ALTER TABLE crm_quotes
    ADD COLUMN IF NOT EXISTS finance_ar_ref TEXT,
    ADD COLUMN IF NOT EXISTS contract_ref TEXT;

CREATE TABLE IF NOT EXISTS crm_export_jobs (
    id          TEXT PRIMARY KEY,
    page        TEXT NOT NULL DEFAULT '',
    range_key   TEXT NOT NULL DEFAULT 'week',
    formats     TEXT NOT NULL DEFAULT 'csv',
    status      TEXT NOT NULL DEFAULT 'queued',
    result_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS crm_seo_audit_jobs (
    id          TEXT PRIMARY KEY,
    url         TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'queued',
    score       INT,
    findings    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS crm_accounts_finance_ref_idx
    ON crm_accounts (finance_customer_ref)
    WHERE finance_customer_ref IS NOT NULL;

COMMIT;
