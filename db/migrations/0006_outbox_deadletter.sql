-- Dead-letter support for the transactional outbox. Rows whose dispatch keeps
-- failing past the retry budget are stamped abandoned_at and skipped by the
-- publisher, instead of retrying forever and clogging the drain query.
ALTER TABLE crm_event_outbox
    ADD COLUMN IF NOT EXISTS abandoned_at TIMESTAMPTZ;

-- Re-scope the pending index so abandoned rows also drop out of the drain query.
DROP INDEX IF EXISTS crm_event_outbox_pending_idx;
CREATE INDEX IF NOT EXISTS crm_event_outbox_pending_idx
    ON crm_event_outbox (available_at)
    WHERE dispatched_at IS NULL AND abandoned_at IS NULL;
