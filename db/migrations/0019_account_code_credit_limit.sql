-- 0019: Human-readable customer codes (CUS-0001) + credit limit on accounts.
--
-- Platform ids stay uuid. The sales CSV / party master uses CUS-#### codes and
-- a credit limit; both were dropped on the way into crm_accounts (code was
-- shown as the uuid id; credit_limit had no column).

ALTER TABLE crm_accounts
  ADD COLUMN IF NOT EXISTS code TEXT,
  ADD COLUMN IF NOT EXISTS credit_limit NUMERIC(18, 2);

-- Empty string and NULL both mean "not issued yet"; only real codes are unique.
CREATE UNIQUE INDEX IF NOT EXISTS crm_accounts_code_uidx
  ON crm_accounts (code)
  WHERE code IS NOT NULL AND btrim(code) <> '';

CREATE INDEX IF NOT EXISTS crm_accounts_code_idx
  ON crm_accounts (code)
  WHERE code IS NOT NULL AND btrim(code) <> '';
