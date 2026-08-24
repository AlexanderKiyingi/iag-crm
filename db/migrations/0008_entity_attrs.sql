-- Free-form overflow for fields a CRM front end collects that this service has
-- no promoted column for.
--
-- Why this exists: the ERP-clone CRM front end (iag-crm) collects roughly
-- fifteen fields the schema has nowhere to put — a lead's pipeline stage,
-- estimated value and next follow-up; a contact's notes and active flag; a
-- follow-up's due date and Planned/Done/Cancelled state; a complaint's
-- resolution notes and resolved date. Every one of them was typed by an
-- operator and then silently dropped on save, because the Patch* allowlists in
-- internal/store are narrower than the read models and neither had a home for
-- them.
--
-- The alternative — promoting each to its own column — bakes one front end's
-- form into the service's schema. `attrs` keeps the service's own model
-- authoritative for everything it does own (stage, status, amount, owner …) and
-- gives client-specific detail a durable place to live, exactly as
-- iag-erp's employees.attrs does.
--
-- NOT a dumping ground: anything this service queries, reports on, or drives
-- workflow from belongs in a real column. Adding one later is a migration that
-- reads out of attrs, not a reason to avoid this.

ALTER TABLE crm_leads      ADD COLUMN IF NOT EXISTS attrs JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE crm_deals      ADD COLUMN IF NOT EXISTS attrs JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE crm_contacts   ADD COLUMN IF NOT EXISTS attrs JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE crm_activities ADD COLUMN IF NOT EXISTS attrs JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE crm_tickets    ADD COLUMN IF NOT EXISTS attrs JSONB NOT NULL DEFAULT '{}'::jsonb;

-- crm_activities has no due date of its own, and a follow-up without one is not
-- a follow-up. Promoted rather than left in attrs because the engagement views
-- and the activity stream need to order and filter on it.
ALTER TABLE crm_activities ADD COLUMN IF NOT EXISTS due_at TIMESTAMPTZ;
ALTER TABLE crm_activities ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS crm_activities_due_at_idx
    ON crm_activities (due_at)
    WHERE due_at IS NOT NULL;

-- A complaint that is closed without a recorded outcome is the common support
-- audit finding, so resolution is promoted alongside the date it happened.
ALTER TABLE crm_tickets ADD COLUMN IF NOT EXISTS resolved_at  TIMESTAMPTZ;
ALTER TABLE crm_tickets ADD COLUMN IF NOT EXISTS resolution   TEXT NOT NULL DEFAULT '';
