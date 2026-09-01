-- 0011: Convert CRM surrogate entity ids from coded text to uuid
--
-- CRM was the only service still minting its own identifiers. Repository.NextID
-- drew from crm_id_counters to produce ACC-500, CON-1200, LEAD-200, DEAL-500 and
-- so on. Those ids become uuid, matching every other service, and the counter
-- table is dropped.
--
-- THIS IS A VISIBLE CHANGE. The ACC-/LEAD-/DEAL- identifiers users saw in the UI
-- and quoted in documents are no longer keys, and no longer appear anywhere in
-- the product. They remain *resolvable*: every coded id is recorded in
-- crm_external_refs against the uuid it became, so a bookmark or a printed
-- document quoting ACC-500 can still be looked up. See the block below.
--
-- WHAT IS DELIBERATELY NOT CONVERTED
--
--   crm_brand_kit.id             Singleton configuration rows keyed by the
--   crm_loyalty_tier_rules.id    literal 'default' and read as WHERE id =
--                                'default' in the store layer. Natural keys,
--                                not entity identity.
--   crm_integration_connections  Composite natural key (provider, user_email).
--   crm_event_outbox.id          bigint, and the relay reads the backlog in id
--                                order; uuid has no ordering.
--   crm_processed_events.event_id   Upstream event ids from other services.
--   crm_api_audit.id             Log tables — a sequential key is the point.
--   crm_journey_step_logs.id
--   crm_accounts.billing_*_id    Ids owned by the billing system.
--   crm_bridge_pending_imports.dms_outlet_id   Owned by DMS.
--   crm_audit_entries.entity_id  Polymorphic — points at any table.
--   crm_external_refs.*          Correlation columns, intentionally text.
--
-- SOFT REFERENCES
--
-- Only 16 of these relationships carry a foreign key. The rest — campaign_id,
-- segment_id, tier_id, lead_id, step_id, converted_deal_id, stream_id — are
-- plain columns, and are converted with the same mapping as the ids they point
-- at or they would silently dangle.

-- RE-RUNNING THIS MIGRATION
--
-- The runner re-executes a recorded migration whenever the ledger's checksum no
-- longer matches the file — a self-heal that assumes every body is idempotent.
-- The first cut of this one was not: it fed an already-uuid column to
-- crm_id_to_uuid(TEXT) and matched a text regex against it, so a re-run died
-- with "function crm_id_to_uuid(uuid) does not exist" before it reached its
-- first ALTER. The re-apply transaction rolled back, the checksum was never
-- re-stamped, and the service crash-looped on every boot with the same mismatch
-- warning. The conversion is therefore state-driven: it looks at the column
-- types it is about to change and does nothing where the work is already done.

-- A table rewrite takes ACCESS EXCLUSIVE. Without a timeout, an ALTER that
-- cannot get the lock immediately waits in the lock queue — and every read that
-- arrives behind it queues too, so a migration run against live traffic stalls
-- the service rather than just itself. Failing after ten seconds leaves the
-- transaction rolled back and the table untouched; re-run it in a quiet window.
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE FUNCTION crm_id_to_uuid(v TEXT) RETURNS UUID AS $fn$
    SELECT CASE
        WHEN v IS NULL OR v = '' THEN NULL
        WHEN v ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            THEN v::uuid
        -- RFC 4122 v3 shape (md5-based) so a coded id maps to a well-formed
        -- uuid, and parents and children land on the same value.
        ELSE uuid_in(overlay(overlay(md5('iag:crm:' || v)
                 placing '3' from 13) placing '8' from 17)::cstring)
    END
$fn$ LANGUAGE sql IMMUTABLE;

DO $convert$
DECLARE
    -- Every table whose surrogate id becomes uuid. The same list drives the
    -- external-ref capture, the type change and the new column default.
    coded_tables TEXT[] := ARRAY[
        'crm_accounts','crm_activities','crm_audit_entries','crm_bridge_field_mappings',
        'crm_bridge_pending_imports','crm_bridge_streams','crm_bridge_sync_log',
        'crm_budget_plans','crm_buying_signals','crm_campaigns','crm_contacts',
        'crm_content_assets','crm_deals','crm_email_sends','crm_events',
        'crm_export_customers','crm_export_jobs','crm_journey_enrollments',
        'crm_journey_steps','crm_journeys','crm_leads','crm_loyalty_outlets',
        'crm_loyalty_promotions','crm_loyalty_tiers','crm_module_records','crm_mqls',
        'crm_outlets','crm_personas','crm_quotes','crm_segments','crm_seo_audit_jobs',
        'crm_seo_keywords','crm_social_posts','crm_tickets'];
    t   TEXT;
    fk  RECORD;
    ref RECORD;
BEGIN
    -- Already converted? Then there is nothing here to do. Resolved with
    -- to_regclass so the check follows the same search_path as the ALTERs below:
    -- a legacy database still carrying these tables in `public` is found, and a
    -- table this database has never had is skipped rather than erroring.
    IF NOT EXISTS (
        SELECT 1
          FROM unnest(coded_tables) AS ct(name)
          JOIN pg_attribute a ON a.attrelid = to_regclass(ct.name)
         WHERE a.attname = 'id'
           AND a.attnum > 0
           AND NOT a.attisdropped
           AND a.atttypid <> 'uuid'::regtype
    ) THEN
        RAISE NOTICE 'crm entity ids are already uuid; nothing to convert';
        RETURN;
    END IF;

    -- ---- keep the coded ids resolvable ------------------------------------
    -- The coded id is not retained as a key, but throwing it away entirely would
    -- strand every bookmark, printed document and downstream record that quotes
    -- ACC-500 or DEAL-500. Each one is recorded in crm_external_refs against the
    -- uuid it became, so a lookup by the old code can still find the row.
    --
    -- Only genuinely coded ids are recorded; anything already stored as a uuid
    -- maps to itself and is not worth a row. source_service is 'crm-legacy-code'
    -- so these are distinguishable from refs pointing at other systems.
    FOREACH t IN ARRAY coded_tables
    LOOP
        CONTINUE WHEN to_regclass(t) IS NULL;
        EXECUTE format($q$
            INSERT INTO crm_external_refs
                (source_service, source_type, source_id, target_type, target_id, origin)
            SELECT 'crm-legacy-code', %L, id::text, %L, crm_id_to_uuid(id::text)::text, 'platform'
              FROM %I
             WHERE id::text !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            ON CONFLICT (source_service, source_type, source_id) DO NOTHING
        $q$, t, t, t);
    END LOOP;

    -- ---- drop the foreign keys --------------------------------------------
    -- Dropped by name with IF EXISTS: on a re-run they were already restored at
    -- the end of the previous pass, and on a database that never carried one the
    -- drop must not be the thing that fails.
    FOR fk IN
        SELECT * FROM (VALUES
            ('crm_activities',          'crm_activities_account_id_fkey'),
            ('crm_activities',          'crm_activities_contact_id_fkey'),
            ('crm_activities',          'crm_activities_deal_id_fkey'),
            ('crm_buying_signals',      'crm_buying_signals_account_id_fkey'),
            ('crm_contacts',            'crm_contacts_account_id_fkey'),
            ('crm_deals',               'crm_deals_account_id_fkey'),
            ('crm_journey_enrollments', 'crm_journey_enrollments_journey_id_fkey'),
            ('crm_journey_step_logs',   'crm_journey_step_logs_enrollment_id_fkey'),
            ('crm_journey_steps',       'crm_journey_steps_journey_id_fkey'),
            ('crm_loyalty_outlets',     'crm_loyalty_outlets_account_id_fkey'),
            ('crm_outlets',             'crm_outlets_account_id_fkey'),
            ('crm_quotes',              'crm_quotes_account_id_fkey'),
            ('crm_quotes',              'crm_quotes_deal_id_fkey'),
            ('crm_tickets',             'crm_tickets_account_id_fkey'),
            ('crm_tickets',             'crm_tickets_contact_id_fkey'),
            ('crm_tickets',             'crm_tickets_deal_id_fkey')
        ) AS v(tbl, name)
    LOOP
        CONTINUE WHEN to_regclass(fk.tbl) IS NULL;
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', fk.tbl, fk.name);
    END LOOP;

    -- ---- primary keys ------------------------------------------------------
    FOREACH t IN ARRAY coded_tables
    LOOP
        CONTINUE WHEN to_regclass(t) IS NULL;
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id::text)', t);
    END LOOP;

    -- ---- referencing columns ----------------------------------------------
    -- Only 16 of these relationships carry a foreign key. The rest — campaign_id,
    -- segment_id, tier_id, lead_id, step_id, converted_deal_id, stream_id — are
    -- plain columns, and are converted with the same mapping as the ids they
    -- point at or they would silently dangle.
    FOR ref IN
        SELECT * FROM (VALUES
            ('crm_activities',            'account_id'),
            ('crm_activities',            'contact_id'),
            ('crm_activities',            'deal_id'),
            ('crm_buying_signals',        'account_id'),
            ('crm_contacts',              'account_id'),
            ('crm_content_assets',        'campaign_id'),
            ('crm_deals',                 'account_id'),
            ('crm_email_sends',           'segment_id'),
            ('crm_journey_enrollments',   'contact_id'),
            ('crm_journey_enrollments',   'journey_id'),
            ('crm_journey_enrollments',   'lead_id'),
            ('crm_journey_step_logs',     'enrollment_id'),
            ('crm_journey_step_logs',     'step_id'),
            ('crm_journey_steps',         'journey_id'),
            ('crm_leads',                 'converted_deal_id'),
            ('crm_loyalty_outlets',       'account_id'),
            ('crm_loyalty_outlets',       'tier_id'),
            ('crm_loyalty_promotions',    'tier_id'),
            ('crm_mqls',                  'campaign_id'),
            ('crm_mqls',                  'lead_id'),
            ('crm_outlets',               'account_id'),
            ('crm_personas',              'journey_id'),
            ('crm_personas',              'segment_id'),
            ('crm_quotes',                'account_id'),
            ('crm_quotes',                'deal_id'),
            ('crm_social_posts',          'campaign_id'),
            ('crm_tickets',               'account_id'),
            ('crm_tickets',               'contact_id'),
            ('crm_tickets',               'deal_id'),
            ('crm_bridge_field_mappings', 'stream_id'),
            ('crm_bridge_sync_log',       'stream_id')
        ) AS v(tbl, col)
    LOOP
        CONTINUE WHEN NOT EXISTS (
            SELECT 1 FROM pg_attribute a
             WHERE a.attrelid = to_regclass(ref.tbl)
               AND a.attname  = ref.col
               AND a.attnum > 0
               AND NOT a.attisdropped
               AND a.atttypid <> 'uuid'::regtype);
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN %I TYPE UUID USING crm_id_to_uuid(%I::text)',
            ref.tbl, ref.col, ref.col);
    END LOOP;

    -- ---- restore the foreign keys ------------------------------------------
    FOR fk IN
        SELECT * FROM (VALUES
            ('crm_activities',          'crm_activities_account_id_fkey',           'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_activities',          'crm_activities_contact_id_fkey',           'contact_id',    'crm_contacts',            'SET NULL'),
            ('crm_activities',          'crm_activities_deal_id_fkey',              'deal_id',       'crm_deals',               'SET NULL'),
            ('crm_buying_signals',      'crm_buying_signals_account_id_fkey',       'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_contacts',            'crm_contacts_account_id_fkey',             'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_deals',               'crm_deals_account_id_fkey',                'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_journey_enrollments', 'crm_journey_enrollments_journey_id_fkey',  'journey_id',    'crm_journeys',            'CASCADE'),
            ('crm_journey_step_logs',   'crm_journey_step_logs_enrollment_id_fkey', 'enrollment_id', 'crm_journey_enrollments', 'CASCADE'),
            ('crm_journey_steps',       'crm_journey_steps_journey_id_fkey',        'journey_id',    'crm_journeys',            'CASCADE'),
            ('crm_loyalty_outlets',     'crm_loyalty_outlets_account_id_fkey',      'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_outlets',             'crm_outlets_account_id_fkey',              'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_quotes',              'crm_quotes_account_id_fkey',               'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_quotes',              'crm_quotes_deal_id_fkey',                  'deal_id',       'crm_deals',               'SET NULL'),
            ('crm_tickets',             'crm_tickets_account_id_fkey',              'account_id',    'crm_accounts',            'SET NULL'),
            ('crm_tickets',             'crm_tickets_contact_id_fkey',              'contact_id',    'crm_contacts',            'SET NULL'),
            ('crm_tickets',             'crm_tickets_deal_id_fkey',                 'deal_id',       'crm_deals',               'SET NULL')
        ) AS v(tbl, name, col, parent, on_delete)
    LOOP
        CONTINUE WHEN to_regclass(fk.tbl) IS NULL OR to_regclass(fk.parent) IS NULL;
        EXECUTE format(
            'ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(id) ON DELETE %s',
            fk.tbl, fk.name, fk.col, fk.parent, fk.on_delete);
    END LOOP;

    -- ---- the database mints ids from here on -------------------------------
    FOREACH t IN ARRAY coded_tables
    LOOP
        CONTINUE WHEN to_regclass(t) IS NULL;
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN id SET DEFAULT gen_random_uuid()', t);
    END LOOP;
END
$convert$;

-- The counter table has no purpose once ids are uuid. Repository.NextID and its
-- callers go with it.
DROP TABLE IF EXISTS crm_id_counters;

DROP FUNCTION IF EXISTS crm_id_to_uuid(TEXT);
