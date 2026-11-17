-- Customer / account outlet geo for CRM desk forms.
-- Distribution already keeps lat/lng on DMS outlets; CRM accounts need the
-- same so a sales user can record where the customer site is without leaving
-- the CRM app.

ALTER TABLE crm_accounts
    ADD COLUMN IF NOT EXISTS latitude  DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
