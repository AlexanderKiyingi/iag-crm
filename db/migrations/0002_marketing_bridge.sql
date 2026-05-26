BEGIN;

CREATE TABLE IF NOT EXISTS crm_segments (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'dynamic',
    refresh     TEXT NOT NULL DEFAULT '5 min',
    rules       TEXT NOT NULL DEFAULT '',
    member_count INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_journeys (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    trigger     TEXT NOT NULL DEFAULT '',
    template    TEXT NOT NULL DEFAULT '',
    goal        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'draft',
    enrolled    INT NOT NULL DEFAULT 0,
    conversion  NUMERIC(5,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_personas (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    buyer_role  TEXT NOT NULL DEFAULT '',
    region      TEXT NOT NULL DEFAULT '',
    seniority   TEXT NOT NULL DEFAULT '',
    segment_id  TEXT,
    journey_id  TEXT,
    content_tags TEXT NOT NULL DEFAULT '',
    story       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_events (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    event_type  TEXT NOT NULL DEFAULT '',
    city        TEXT NOT NULL DEFAULT '',
    starts_on   DATE,
    ends_on     DATE,
    budget_usd  NUMERIC(18,2),
    mql_target  INT NOT NULL DEFAULT 0,
    registrations INT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'planned',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_content_assets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    asset_type  TEXT NOT NULL DEFAULT '',
    format      TEXT NOT NULL DEFAULT '',
    campaign_id TEXT,
    tags        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'draft',
    usage_count INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_email_sends (
    id          TEXT PRIMARY KEY,
    subject     TEXT NOT NULL,
    template    TEXT NOT NULL DEFAULT '',
    segment_id  TEXT,
    body        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'draft',
    scheduled_at TIMESTAMPTZ,
    open_rate   NUMERIC(5,2),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_social_posts (
    id          TEXT PRIMARY KEY,
    platforms   TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    campaign_id TEXT,
    status      TEXT NOT NULL DEFAULT 'draft',
    scheduled_at TIMESTAMPTZ,
    engagement  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_seo_keywords (
    id          TEXT PRIMARY KEY,
    term        TEXT NOT NULL,
    intent      TEXT NOT NULL DEFAULT '',
    locale      TEXT NOT NULL DEFAULT '',
    landing_page TEXT NOT NULL DEFAULT '',
    rank        INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_budget_plans (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    quarter     TEXT NOT NULL DEFAULT '',
    owner       TEXT NOT NULL DEFAULT '',
    channels    JSONB NOT NULL DEFAULT '{}'::jsonb,
    mql_target  INT NOT NULL DEFAULT 0,
    sql_target  INT NOT NULL DEFAULT 0,
    won_target  NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_mqls (
    id          TEXT PRIMARY KEY,
    lead_id     TEXT,
    name        TEXT NOT NULL,
    company     TEXT NOT NULL DEFAULT '',
    score       INT NOT NULL DEFAULT 0,
    source      TEXT NOT NULL DEFAULT '',
    campaign_id TEXT,
    owner       TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'new',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_brand_kit (
    id          TEXT PRIMARY KEY DEFAULT 'default',
    colors      JSONB NOT NULL DEFAULT '{}'::jsonb,
    fonts       JSONB NOT NULL DEFAULT '{}'::jsonb,
    voice       TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_loyalty_tiers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    min_points  INT NOT NULL DEFAULT 0,
    benefits    TEXT NOT NULL DEFAULT '',
    multiplier  NUMERIC(4,2) NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_loyalty_outlets (
    id          TEXT PRIMARY KEY,
    dms_ref     TEXT,
    account_id  TEXT REFERENCES crm_accounts(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    tier_id     TEXT,
    points      INT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_loyalty_promotions (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    tier_id     TEXT,
    discount    TEXT NOT NULL DEFAULT '',
    starts_on   DATE,
    ends_on     DATE,
    status      TEXT NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_bridge_streams (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    direction   TEXT NOT NULL DEFAULT 'inbound',
    status      TEXT NOT NULL DEFAULT 'connected',
    last_sync_at TIMESTAMPTZ,
    record_count INT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS crm_bridge_pending_imports (
    id          TEXT PRIMARY KEY,
    dms_outlet_id TEXT NOT NULL,
    channel     TEXT NOT NULL DEFAULT '',
    beat        TEXT NOT NULL DEFAULT '',
    assigned_owner TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_bridge_field_mappings (
    id          TEXT PRIMARY KEY,
    stream_id   TEXT NOT NULL,
    dms_field   TEXT NOT NULL,
    crm_field   TEXT NOT NULL,
    transform   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS crm_bridge_sync_log (
    id          TEXT PRIMARY KEY,
    stream_id   TEXT NOT NULL,
    level       TEXT NOT NULL DEFAULT 'info',
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_outlets (
    id          TEXT PRIMARY KEY,
    dms_ref     TEXT UNIQUE,
    account_id  TEXT REFERENCES crm_accounts(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    city        TEXT NOT NULL DEFAULT '',
    segment     TEXT NOT NULL DEFAULT '',
    health      INT NOT NULL DEFAULT 0,
    owner       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS crm_export_customers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    country     TEXT NOT NULL DEFAULT '',
    currency    TEXT NOT NULL DEFAULT 'USD',
    incoterms   TEXT NOT NULL DEFAULT '',
    credit_limit NUMERIC(18,2),
    erp_ref     TEXT,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
