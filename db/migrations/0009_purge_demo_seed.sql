-- Purge the CRM prototype dataset written by seed.Run (internal/seed/seed.go).
--
-- Every row the seed wrote is named explicitly rather than matched by prefix: new
-- records draw their ids from crm_id_counters and land in the same ACC-/CON-/DEAL-
-- namespace, so a prefix match could not tell demo rows from an operator's.
--
-- Deliberately preserved:
--   * crm_id_counters — the id sequence state; resetting it would hand new records
--     ids that collide with anything created since.
--   * crm_loyalty_tier_rules (0003_audit_ops.sql) and crm_loyalty_tiers /
--     crm_brand_kit — loyalty tier definitions and brand assets are configuration.
--
-- The runner wraps each migration in its own transaction, so this file does not open
-- one: a COMMIT here would end that transaction early.
--
-- The ids are compared as text because the runner replays a recorded migration
-- whenever its checksum drifts, and by then 0011_uuid_entity_ids has retyped these
-- columns to uuid. 'ACT-1001' is not a uuid, so an uncast comparison aborts the
-- boot with 22P02 instead of matching nothing. On a database that still holds the
-- demo rows the columns are text and the cast changes neither the rows matched nor
-- the plan's ability to use the primary key.

-- ---- dependents of accounts / contacts / deals ----------------------------
DELETE FROM crm_activities WHERE id::text IN (
    'ACT-1001', 'ACT-1002'
);

DELETE FROM crm_quotes WHERE id::text IN (
    'QTE-0418', 'QTE-0421'
);

DELETE FROM crm_tickets WHERE id::text IN (
    'TKT-0094', 'TKT-0218'
);

DELETE FROM crm_buying_signals WHERE id::text IN (
    'SIG-001', 'SIG-002', 'SIG-003'
);

DELETE FROM crm_outlets WHERE id::text IN (
    'OUT-0892', 'OUT-1247'
);

-- ---- journeys: enrolments → step logs → steps → journeys ------------------
DELETE FROM crm_journey_step_logs
WHERE enrollment_id IN (
    SELECT id FROM crm_journey_enrollments WHERE journey_id::text IN ('JRN-001', 'JRN-002')
);

DELETE FROM crm_journey_enrollments WHERE journey_id::text IN ('JRN-001', 'JRN-002');
DELETE FROM crm_journey_steps       WHERE journey_id::text IN ('JRN-001', 'JRN-002');
DELETE FROM crm_journeys WHERE id::text IN (
    'JRN-001', 'JRN-002'
);

-- ---- marketing bridge (all standalone) ------------------------------------
DELETE FROM crm_bridge_streams WHERE id::text IN (
    'STR-01', 'STR-02', 'STR-03', 'STR-04',
    'STR-05', 'STR-06'
);

DELETE FROM crm_segments WHERE id::text IN (
    'SEG-001', 'SEG-002', 'SEG-003'
);

DELETE FROM crm_campaigns WHERE id::text IN (
    'CMP-0038', 'CMP-0042'
);

DELETE FROM crm_email_sends WHERE id::text IN (
    'EML-001', 'EML-002'
);

DELETE FROM crm_social_posts WHERE id::text IN (
    'SOC-001'
);

DELETE FROM crm_personas WHERE id::text IN (
    'PSN-001', 'PSN-002'
);

DELETE FROM crm_mqls WHERE id::text IN (
    'MQL-001', 'MQL-002'
);

DELETE FROM crm_events WHERE id::text IN (
    'EVT-001', 'EVT-002'
);

DELETE FROM crm_budget_plans WHERE id::text IN (
    'BDG-001'
);

DELETE FROM crm_export_customers WHERE id::text IN (
    'EXP-001', 'EXP-002'
);

DELETE FROM crm_leads WHERE id::text IN (
    'LEAD-0111', 'LEAD-0114', 'LEAD-0117'
);

-- ---- core objects, parents last -------------------------------------------
DELETE FROM crm_contacts WHERE id::text IN (
    'CON-1099', 'CON-1102', 'CON-1105', 'CON-1108',
    'CON-1111', 'CON-1114', 'CON-1117', 'CON-1120',
    'CON-1123', 'CON-1126', 'CON-1129', 'CON-1132',
    'CON-1135', 'CON-1138', 'CON-1142'
);

DELETE FROM crm_deals WHERE id::text IN (
    'DEAL-0411', 'DEAL-0415', 'DEAL-0418', 'DEAL-0419',
    'DEAL-0420', 'DEAL-0421', 'DEAL-0422', 'DEAL-0426',
    'DEAL-0429', 'DEAL-0431'
);

DELETE FROM crm_accounts WHERE id::text IN (
    'ACC-0368', 'ACC-0371', 'ACC-0374', 'ACC-0377',
    'ACC-0380', 'ACC-0383', 'ACC-0386', 'ACC-0389',
    'ACC-0392', 'ACC-0395', 'ACC-0398', 'ACC-0403',
    'ACC-0406', 'ACC-0409', 'ACC-0412', 'ACC-0415',
    'ACC-0418', 'ACC-0421'
);
