-- Campaign CEO approval vocabulary.
-- draft → pending_ceo → approved → live (then paused / completed).
-- Legacy "planned" rows become draft so marketing hub and desk agree.

UPDATE crm_campaigns
SET status = 'draft'
WHERE lower(trim(status)) IN ('planned', '');

UPDATE crm_campaigns
SET status = lower(replace(trim(status), ' ', '_'))
WHERE status IS NOT NULL AND status <> lower(replace(trim(status), ' ', '_'));

COMMENT ON COLUMN crm_campaigns.status IS
  'Controlled: draft | pending_ceo | approved | live | paused | completed';
