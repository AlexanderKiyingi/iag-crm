-- Link campaigns to a CRM segment (membership = live segment; audience stays a label).

ALTER TABLE crm_campaigns
  ADD COLUMN IF NOT EXISTS segment_id TEXT;

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
