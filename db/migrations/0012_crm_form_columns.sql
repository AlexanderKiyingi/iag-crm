-- 0012: Promote the front-end fields that a query has to be able to see.
--
-- 0008 introduced `attrs` for client fields this service has no column for, and
-- drew the line deliberately: anything the service queries, reports on, or
-- drives workflow from belongs in a real column, and moving one out of attrs
-- later is a migration that reads out of attrs. This is that migration.
--
-- Four things forced it, all found by round-tripping the live app rather than
-- by reading the schema:
--
--   1. A complaint could not be dated. `crm_tickets` has created_at and nothing
--      else, so a complaint logged on Monday for an incident that happened on
--      Friday was recorded as Monday's. The front end has always had a required
--      "Date" field; it had nowhere to go and was dropped on every save.
--
--   2. A lead's estimated value and next follow-up sat in attrs, where no
--      query can reach them. "Which leads are worth chasing this week" is the
--      one question a lead list exists to answer, and it needed a table scan
--      with a JSONB extract to answer it.
--
--   3. A lead had no account link at all. Deals, contacts, activities and
--      tickets all resolve a free-text customer name to account_id on write;
--      leads alone kept the name in attrs, so a lead never appeared on the
--      account 360 of the company it belonged to.
--
--   4. crm_deals.won_at was written only by SetDealStage. A deal closed
--      through a plain PATCH — which is what the app does, and which already
--      runs finalizeDealWon — left won_at NULL, and PipelineSummary computes
--      velocity as AVG(COALESCE(won_at, NOW()) - created_at). Every deal the
--      app won kept ageing, so average sales-cycle length drifted upward for
--      ever. That fix is in PatchDeal; the backfill below repairs the rows it
--      already happened to.
--
-- Everything here is additive and backfilled from attrs, so a row written
-- before this migration keeps its value. The attrs keys are left in place: they
-- are harmless, and deleting them would make a rollback lossy.

-- ── 1. complaints are dated by when they happened ─────────────────────────
--
-- Backfilled from created_at rather than left NULL: for every existing row the
-- report date genuinely is the row's creation date, and a NULL here would make
-- "undated" indistinguishable from "logged before we could record it".
ALTER TABLE crm_tickets ADD COLUMN IF NOT EXISTS occurred_at TIMESTAMPTZ;
UPDATE crm_tickets SET occurred_at = created_at WHERE occurred_at IS NULL;

CREATE INDEX IF NOT EXISTS crm_tickets_occurred_at_idx
    ON crm_tickets (occurred_at DESC)
    WHERE occurred_at IS NOT NULL;

-- ── 2. lead value and follow-up date become queryable ─────────────────────
ALTER TABLE crm_leads ADD COLUMN IF NOT EXISTS next_follow_up_at TIMESTAMPTZ;
ALTER TABLE crm_leads ADD COLUMN IF NOT EXISTS estimated_value   NUMERIC(18,2);
ALTER TABLE crm_leads ADD COLUMN IF NOT EXISTS currency          TEXT NOT NULL DEFAULT '';

-- ── 3. a lead belongs to an account, like everything else here ────────────
ALTER TABLE crm_leads ADD COLUMN IF NOT EXISTS account_id   UUID REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_leads ADD COLUMN IF NOT EXISTS account_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS crm_leads_account_id_idx
    ON crm_leads (account_id)
    WHERE account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS crm_leads_next_follow_up_at_idx
    ON crm_leads (next_follow_up_at)
    WHERE next_follow_up_at IS NOT NULL;

-- Backfill out of attrs, which is where the app has been writing all four since
-- 0008. Only well-formed values move: a date that does not parse and a value
-- that is not a number stay in attrs rather than becoming a wrong column.
UPDATE crm_leads
   SET next_follow_up_at = (attrs->>'nextFollowUp')::timestamptz
 WHERE next_follow_up_at IS NULL
   AND attrs->>'nextFollowUp' ~ '^\d{4}-\d{2}-\d{2}';

UPDATE crm_leads
   SET estimated_value = (attrs->>'estimatedValue')::numeric
 WHERE estimated_value IS NULL
   AND attrs->>'estimatedValue' ~ '^-?\d+(\.\d+)?$';

UPDATE crm_leads
   SET currency = attrs->>'currency'
 WHERE currency = ''
   AND COALESCE(attrs->>'currency', '') <> '';

UPDATE crm_leads
   SET account_name = attrs->>'customer'
 WHERE account_name = ''
   AND COALESCE(attrs->>'customer', '') <> '';

-- Link the backfilled names to real accounts where one matches, case-insensitively
-- and on the trimmed name — the same rule resolveAccountID applies on write.
UPDATE crm_leads l
   SET account_id = a.id
  FROM crm_accounts a
 WHERE l.account_id IS NULL
   AND l.account_name <> ''
   AND LOWER(TRIM(a.name)) = LOWER(TRIM(l.account_name));

-- ── 4. repair deals won through a plain PATCH ─────────────────────────────
--
-- won_at is unknown for these rows; updated_at is the closest honest proxy,
-- being the timestamp of the write that set the stage to won. It is used only
-- where won_at would otherwise read as NOW(), which is strictly worse.
UPDATE crm_deals SET won_at = updated_at WHERE stage = 'won' AND won_at IS NULL;
