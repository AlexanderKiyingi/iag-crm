-- Org / owner sales quotas for the CRM dashboard "Sales vs target" widget.
-- Actuals are rolled up from won deals (crm_deals.stage = 'won') for the period.

CREATE TABLE IF NOT EXISTS crm_sales_targets (
   id          TEXT PRIMARY KEY,
   name        TEXT NOT NULL,
   period      TEXT NOT NULL DEFAULT '',          -- e.g. 2026-Q4 or 2026-11
   target_usd  NUMERIC(18, 2) NOT NULL DEFAULT 0,
   owner       TEXT NOT NULL DEFAULT '',         -- empty = organisation-wide
   status      TEXT NOT NULL DEFAULT 'active',   -- active | closed
   created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
   updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_sales_targets_period_idx
   ON crm_sales_targets (period);

CREATE INDEX IF NOT EXISTS crm_sales_targets_status_idx
   ON crm_sales_targets (status);

COMMENT ON TABLE crm_sales_targets IS
 'Sales quotas; progress endpoint compares won deal amounts for the period';
