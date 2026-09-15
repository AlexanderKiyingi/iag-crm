-- 0013: Controlled vocabularies for the columns a query reads, and a real
-- status column on contacts.
--
-- Deal stage and lead status already had this treatment: the client's display
-- words were being written raw into a column the service filters on, so the
-- filter matched nothing. Ticket status and priority and activity status were
-- on the same path — the app writes "Open", "In progress", "Low", "Planned" —
-- and nothing in the service queried them yet, which is the only reason it had
-- not gone wrong. This migration normalises what is already stored and the
-- handlers now refuse anything outside models/vocab.go.
--
-- Contacts gain `status` because Active/Inactive lived in attrs, where journey
-- enrolment and the lookups could not see it: an inactive contact was still
-- auto-enrolled and still offered in every picker.
--
-- Every statement is idempotent, so a replay is a no-op.

-- ── tickets ─────────────────────────────────────────────────────────────
UPDATE crm_tickets SET status = CASE LOWER(REPLACE(status, ' ', '_'))
    WHEN 'open'        THEN 'open'
    WHEN 'in_progress' THEN 'in_progress'
    WHEN 'resolved'    THEN 'resolved'
    WHEN 'closed'      THEN 'closed'
    ELSE status END
WHERE status <> LOWER(REPLACE(status, ' ', '_'));

UPDATE crm_tickets SET priority = CASE LOWER(priority)
    WHEN 'urgent' THEN 'P1'
    WHEN 'high'   THEN 'P2'
    WHEN 'medium' THEN 'P3'
    WHEN 'low'    THEN 'P4'
    WHEN 'p1'     THEN 'P1'
    WHEN 'p2'     THEN 'P2'
    WHEN 'p3'     THEN 'P3'
    WHEN 'p4'     THEN 'P4'
    WHEN ''       THEN 'P2'
    ELSE priority END
WHERE priority NOT IN ('P1','P2','P3','P4');

UPDATE crm_tickets SET channel = LOWER(channel) WHERE channel <> LOWER(channel);

-- ── activities ──────────────────────────────────────────────────────────
UPDATE crm_activities SET status = CASE LOWER(status)
    WHEN 'planned'   THEN 'planned'
    WHEN 'done'      THEN 'done'
    WHEN 'completed' THEN 'done'
    WHEN 'cancelled' THEN 'cancelled'
    WHEN 'canceled'  THEN 'cancelled'
    WHEN ''          THEN 'planned'
    ELSE status END
WHERE status IS NULL OR status NOT IN ('planned','done','cancelled');

-- ── contacts ────────────────────────────────────────────────────────────
ALTER TABLE crm_contacts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

-- Backfill from the attrs key the app has been writing. The key is left in
-- attrs, as 0012 did: harmless, and a rollback stays lossless.
UPDATE crm_contacts
   SET status = CASE LOWER(COALESCE(attrs->>'status', ''))
                    WHEN 'inactive' THEN 'inactive'
                    ELSE 'active' END
 WHERE status = 'active' AND LOWER(COALESCE(attrs->>'status', '')) = 'inactive';

CREATE INDEX IF NOT EXISTS crm_contacts_status_idx ON crm_contacts (status);

-- ── currency codes ──────────────────────────────────────────────────────
-- The overview sums by currency; "ugx" beside "UGX" is two buckets.
UPDATE crm_leads SET currency = UPPER(TRIM(currency)) WHERE currency <> UPPER(TRIM(currency));
UPDATE crm_deals SET currency = UPPER(TRIM(currency)) WHERE currency <> UPPER(TRIM(currency));
