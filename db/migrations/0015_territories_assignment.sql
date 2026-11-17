BEGIN;

CREATE TABLE IF NOT EXISTS crm_territories (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    region       TEXT NOT NULL DEFAULT '',
    owner_email  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_territories_region_idx ON crm_territories (LOWER(region));

CREATE TABLE IF NOT EXISTS crm_assignment_rules (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    region       TEXT NOT NULL DEFAULT '',
    segment      TEXT NOT NULL DEFAULT '',
    min_score    INT NOT NULL DEFAULT 0,
    owner_email  TEXT NOT NULL DEFAULT '',
    priority     INT NOT NULL DEFAULT 100,
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_assignment_rules_active_idx
    ON crm_assignment_rules (active, priority ASC);

COMMIT;
