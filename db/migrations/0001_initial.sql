BEGIN;

CREATE TABLE IF NOT EXISTS crm_id_counters (
    prefix     TEXT PRIMARY KEY,
    next_value BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS crm_accounts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    account_type    TEXT NOT NULL DEFAULT 'Export',
    country         TEXT NOT NULL DEFAULT '',
    segment         TEXT NOT NULL DEFAULT '',
    owner           TEXT NOT NULL DEFAULT '',
    value_display   TEXT NOT NULL DEFAULT '',
    value_amount    NUMERIC(18, 2),
    value_currency  TEXT,
    health_score    INT NOT NULL DEFAULT 0 CHECK (health_score >= 0 AND health_score <= 100),
    status          TEXT NOT NULL DEFAULT 'active',
    dms_bridged     BOOLEAN NOT NULL DEFAULT FALSE,
    dms_ref         TEXT,
    email           TEXT,
    phone           TEXT,
    address         TEXT,
    last_touch_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_accounts_owner_idx ON crm_accounts (owner);
CREATE INDEX IF NOT EXISTS crm_accounts_status_idx ON crm_accounts (status);
CREATE INDEX IF NOT EXISTS crm_accounts_dms_ref_idx ON crm_accounts (dms_ref) WHERE dms_ref IS NOT NULL;

CREATE TABLE IF NOT EXISTS crm_contacts (
    id              TEXT PRIMARY KEY,
    account_id      TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name    TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    buyer_role      TEXT NOT NULL DEFAULT '',
    owner           TEXT NOT NULL DEFAULT '',
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_contacts_account_id_idx ON crm_contacts (account_id);
CREATE INDEX IF NOT EXISTS crm_contacts_owner_idx ON crm_contacts (owner);

CREATE TABLE IF NOT EXISTS crm_leads (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    company         TEXT NOT NULL DEFAULT '',
    email           TEXT NOT NULL DEFAULT '',
    phone           TEXT NOT NULL DEFAULT '',
    source          TEXT NOT NULL DEFAULT '',
    segment         TEXT NOT NULL DEFAULT '',
    region          TEXT NOT NULL DEFAULT '',
    score           INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'qualifying',
    owner           TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    converted_deal_id TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_leads_owner_idx ON crm_leads (owner);
CREATE INDEX IF NOT EXISTS crm_leads_status_idx ON crm_leads (status);

CREATE TABLE IF NOT EXISTS crm_deals (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    account_id      TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name    TEXT NOT NULL DEFAULT '',
    stage           TEXT NOT NULL DEFAULT 'lead',
    probability     INT NOT NULL DEFAULT 0 CHECK (probability >= 0 AND probability <= 100),
    owner           TEXT NOT NULL DEFAULT '',
    currency        TEXT NOT NULL DEFAULT 'USD',
    amount          NUMERIC(18, 2) NOT NULL DEFAULT 0,
    amount_display  TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    source          TEXT NOT NULL DEFAULT '',
    dms_linked      BOOLEAN NOT NULL DEFAULT FALSE,
    close_date      DATE,
    notes           TEXT NOT NULL DEFAULT '',
    won_at          TIMESTAMPTZ,
    lost_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_deals_stage_idx ON crm_deals (stage);
CREATE INDEX IF NOT EXISTS crm_deals_owner_idx ON crm_deals (owner);
CREATE INDEX IF NOT EXISTS crm_deals_account_id_idx ON crm_deals (account_id);

CREATE TABLE IF NOT EXISTS crm_quotes (
    id              TEXT PRIMARY KEY,
    ref             TEXT NOT NULL UNIQUE,
    account_id      TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name    TEXT NOT NULL DEFAULT '',
    deal_id         TEXT REFERENCES crm_deals (id) ON DELETE SET NULL,
    template        TEXT NOT NULL DEFAULT '',
    currency        TEXT NOT NULL DEFAULT 'USD',
    incoterms       TEXT NOT NULL DEFAULT '',
    payment_terms   TEXT NOT NULL DEFAULT '',
    valid_until     DATE,
    total           NUMERIC(18, 2) NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'draft',
    version         INT NOT NULL DEFAULT 1,
    owner           TEXT NOT NULL DEFAULT '',
    line_items      JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_quotes_deal_id_idx ON crm_quotes (deal_id);
CREATE INDEX IF NOT EXISTS crm_quotes_account_id_idx ON crm_quotes (account_id);

CREATE TABLE IF NOT EXISTS crm_activities (
    id              TEXT PRIMARY KEY,
    activity_type   TEXT NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL DEFAULT '',
    account_id      TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name    TEXT NOT NULL DEFAULT '',
    contact_id      TEXT REFERENCES crm_contacts (id) ON DELETE SET NULL,
    deal_id         TEXT REFERENCES crm_deals (id) ON DELETE SET NULL,
    outlet_ref      TEXT,
    owner           TEXT NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_activities_occurred_at_idx ON crm_activities (occurred_at DESC);
CREATE INDEX IF NOT EXISTS crm_activities_deal_id_idx ON crm_activities (deal_id);

CREATE TABLE IF NOT EXISTS crm_tickets (
    id              TEXT PRIMARY KEY,
    account_id      TEXT REFERENCES crm_accounts (id) ON DELETE SET NULL,
    account_name    TEXT NOT NULL DEFAULT '',
    contact_id      TEXT REFERENCES crm_contacts (id) ON DELETE SET NULL,
    outlet_ref      TEXT,
    deal_id         TEXT REFERENCES crm_deals (id) ON DELETE SET NULL,
    subject         TEXT NOT NULL,
    ticket_type     TEXT NOT NULL DEFAULT '',
    priority        TEXT NOT NULL DEFAULT 'P2',
    channel         TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'open',
    owner           TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    dms_claim_ref   TEXT,
    sla_due_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_tickets_status_idx ON crm_tickets (status);
CREATE INDEX IF NOT EXISTS crm_tickets_priority_idx ON crm_tickets (priority);

CREATE TABLE IF NOT EXISTS crm_campaigns (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    campaign_type   TEXT NOT NULL DEFAULT '',
    audience        TEXT NOT NULL DEFAULT '',
    starts_on       DATE,
    ends_on         DATE,
    budget_usd      NUMERIC(18, 2),
    owner           TEXT NOT NULL DEFAULT '',
    goal            TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
