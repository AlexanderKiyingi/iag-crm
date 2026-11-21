-- Link campaigns to a CRM segment (membership = live segment; audience stays a label).
--
-- crm_segments.id became UUID in 0011. segment_id must match that type or
-- Postgres rejects the FK with SQLSTATE 42804 (datatype mismatch).

ALTER TABLE crm_campaigns
  ADD COLUMN IF NOT EXISTS segment_id UUID;

-- If an earlier cut of this file created the column as TEXT and the FK failed
-- mid-transaction on some runners that commit DDL early, coerce it.
DO $type$
BEGIN
  IF EXISTS (
    SELECT 1
      FROM information_schema.columns
     WHERE table_name = 'crm_campaigns'
       AND column_name = 'segment_id'
       AND data_type = 'text'
  ) THEN
    ALTER TABLE crm_campaigns
      ALTER COLUMN segment_id TYPE UUID
      USING NULLIF(btrim(segment_id), '')::uuid;
  END IF;
END
$type$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'crm_campaigns_segment_id_fkey'
  ) THEN
    ALTER TABLE crm_campaigns
      ADD CONSTRAINT crm_campaigns_segment_id_fkey
      FOREIGN KEY (segment_id) REFERENCES crm_segments (id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS crm_campaigns_segment_id_idx
  ON crm_campaigns (segment_id)
  WHERE segment_id IS NOT NULL;

COMMENT ON COLUMN crm_campaigns.segment_id IS
  'Optional CRM segment; audience remains the display label for the list.';
