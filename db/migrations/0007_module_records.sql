-- Generic per-module record store for lightweight, CRM-owned modules that have
-- no dedicated backend of their own: Products, Price Books, Services, Solutions,
-- Projects, Voice of the Customer and Documents. Each row keeps the display
-- fields the UI sorts/filters on (name/owner/status) plus the full submitted form
-- as a JSONB blob, so new modules need no schema change.
--
-- Financial and procurement modules (Invoices, Sales Orders, Purchase Orders,
-- Vendors) are intentionally NOT stored here — those domains are owned by
-- iag-finance and iag-procurement, and duplicating them in CRM would fork the
-- system of record. They stay client-local until wired to their owning service.
CREATE TABLE IF NOT EXISTS crm_module_records (
    id           TEXT PRIMARY KEY,
    module       TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    owner        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT '',
    field_values JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_module_records_module_idx
    ON crm_module_records (module, created_at DESC);
