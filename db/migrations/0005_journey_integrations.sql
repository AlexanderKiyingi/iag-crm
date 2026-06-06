BEGIN;

-- Journey execution engine
CREATE TABLE IF NOT EXISTS crm_journey_steps (
    id           TEXT PRIMARY KEY,
    journey_id   TEXT NOT NULL REFERENCES crm_journeys (id) ON DELETE CASCADE,
    step_order   INT NOT NULL,
    step_type    TEXT NOT NULL DEFAULT 'wait',
    title        TEXT NOT NULL DEFAULT '',
    config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    delay_hours  INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (journey_id, step_order)
);

CREATE INDEX IF NOT EXISTS crm_journey_steps_journey_idx ON crm_journey_steps (journey_id, step_order);

CREATE TABLE IF NOT EXISTS crm_journey_enrollments (
    id            TEXT PRIMARY KEY,
    journey_id    TEXT NOT NULL REFERENCES crm_journeys (id) ON DELETE CASCADE,
    contact_id    TEXT,
    lead_id       TEXT,
    subject_email TEXT NOT NULL DEFAULT '',
    subject_name  TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'active',
    current_step  INT NOT NULL DEFAULT 1,
    next_run_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    enrolled_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS crm_journey_enrollments_due_idx
    ON crm_journey_enrollments (next_run_at)
    WHERE status = 'active';

CREATE TABLE IF NOT EXISTS crm_journey_step_logs (
    id             BIGSERIAL PRIMARY KEY,
    enrollment_id  TEXT NOT NULL REFERENCES crm_journey_enrollments (id) ON DELETE CASCADE,
    step_id        TEXT,
    step_order     INT NOT NULL,
    step_type      TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'ok',
    detail         TEXT NOT NULL DEFAULT '',
    executed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Email / calendar OAuth connections (per CRM user email)
CREATE TABLE IF NOT EXISTS crm_integration_connections (
    user_email        TEXT NOT NULL,
    provider          TEXT NOT NULL,
    provider_user_id  TEXT NOT NULL DEFAULT '',
    access_token      TEXT NOT NULL,
    refresh_token     TEXT NOT NULL DEFAULT '',
    token_expires_at  TIMESTAMPTZ,
    scopes            TEXT NOT NULL DEFAULT '',
    calendar_synced_at TIMESTAMPTZ,
    email_synced_at   TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_email, provider)
);

COMMIT;
