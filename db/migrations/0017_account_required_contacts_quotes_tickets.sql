-- Contacts, quotes and tickets belong to a customer.
-- Backfill account_id from account_name, park irrecoverable orphans on a
-- system account, then enforce NOT NULL + ON DELETE RESTRICT.

-- Stable UUID for the migration holding account (not a real customer).
-- uuid_generate is unavailable in all envs; use a fixed nil-ish v4 shape.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM crm_accounts WHERE id = '00000000-0000-4000-8000-0000000000aa'
  ) THEN
    INSERT INTO crm_accounts (
      id, name, account_type, country, segment, owner, value_display, status, created_at, updated_at
    ) VALUES (
      '00000000-0000-4000-8000-0000000000aa',
      'Unassigned (legacy migration)',
      'Other',
      '',
      '',
      'system',
      '',
      'active',
      NOW(),
      NOW()
    );
  END IF;
END $$;

-- Backfill from exact account_name match.
UPDATE crm_contacts c
SET account_id = a.id
FROM crm_accounts a
WHERE c.account_id IS NULL
  AND TRIM(COALESCE(c.account_name, '')) <> ''
  AND a.name = c.account_name;

UPDATE crm_quotes q
SET account_id = a.id
FROM crm_accounts a
WHERE q.account_id IS NULL
  AND TRIM(COALESCE(q.account_name, '')) <> ''
  AND a.name = q.account_name;

UPDATE crm_tickets t
SET account_id = a.id
FROM crm_accounts a
WHERE t.account_id IS NULL
  AND TRIM(COALESCE(t.account_name, '')) <> ''
  AND a.name = t.account_name;

-- Remaining orphans → holding account; keep their display name if set.
UPDATE crm_contacts
SET account_id = '00000000-0000-4000-8000-0000000000aa',
    account_name = CASE
      WHEN TRIM(COALESCE(account_name, '')) = '' THEN 'Unassigned (legacy migration)'
      ELSE account_name
    END
WHERE account_id IS NULL;

UPDATE crm_quotes
SET account_id = '00000000-0000-4000-8000-0000000000aa',
    account_name = CASE
      WHEN TRIM(COALESCE(account_name, '')) = '' THEN 'Unassigned (legacy migration)'
      ELSE account_name
    END
WHERE account_id IS NULL;

UPDATE crm_tickets
SET account_id = '00000000-0000-4000-8000-0000000000aa',
    account_name = CASE
      WHEN TRIM(COALESCE(account_name, '')) = '' THEN 'Unassigned (legacy migration)'
      ELSE account_name
    END
WHERE account_id IS NULL;

-- Replace SET NULL FKs with RESTRICT + NOT NULL.
ALTER TABLE crm_contacts DROP CONSTRAINT IF EXISTS crm_contacts_account_id_fkey;
ALTER TABLE crm_quotes DROP CONSTRAINT IF EXISTS crm_quotes_account_id_fkey;
ALTER TABLE crm_tickets DROP CONSTRAINT IF EXISTS crm_tickets_account_id_fkey;

ALTER TABLE crm_contacts ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE crm_quotes ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE crm_tickets ALTER COLUMN account_id SET NOT NULL;

ALTER TABLE crm_contacts
  ADD CONSTRAINT crm_contacts_account_id_fkey
  FOREIGN KEY (account_id) REFERENCES crm_accounts (id) ON DELETE RESTRICT;

ALTER TABLE crm_quotes
  ADD CONSTRAINT crm_quotes_account_id_fkey
  FOREIGN KEY (account_id) REFERENCES crm_accounts (id) ON DELETE RESTRICT;

ALTER TABLE crm_tickets
  ADD CONSTRAINT crm_tickets_account_id_fkey
  FOREIGN KEY (account_id) REFERENCES crm_accounts (id) ON DELETE RESTRICT;

COMMENT ON COLUMN crm_contacts.account_id IS
  'Required customer; ON DELETE RESTRICT so contacts are not silently orphaned.';
COMMENT ON COLUMN crm_quotes.account_id IS
  'Required customer; ON DELETE RESTRICT.';
COMMENT ON COLUMN crm_tickets.account_id IS
  'Required customer; ON DELETE RESTRICT.';
