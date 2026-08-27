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

-- ---- keep the coded ids resolvable ----------------------------------------
-- The coded id is not retained as a key, but throwing it away entirely would
-- strand every bookmark, printed document and downstream record that quotes
-- ACC-500 or DEAL-500. Each one is recorded in crm_external_refs against the
-- uuid it became, so a lookup by the old code can still find the row.
--
-- Only genuinely coded ids are recorded; anything already stored as a uuid maps
-- to itself and is not worth a row. source_service is 'crm-legacy-code' so these
-- are distinguishable from refs pointing at other systems.
DO $legacy$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'crm_accounts','crm_activities','crm_audit_entries','crm_bridge_field_mappings',
        'crm_bridge_pending_imports','crm_bridge_streams','crm_bridge_sync_log',
        'crm_budget_plans','crm_buying_signals','crm_campaigns','crm_contacts',
        'crm_content_assets','crm_deals','crm_email_sends','crm_events',
        'crm_export_customers','crm_export_jobs','crm_journey_enrollments',
        'crm_journey_steps','crm_journeys','crm_leads','crm_loyalty_outlets',
        'crm_loyalty_promotions','crm_loyalty_tiers','crm_module_records','crm_mqls',
        'crm_outlets','crm_personas','crm_quotes','crm_segments','crm_seo_audit_jobs',
        'crm_seo_keywords','crm_social_posts','crm_tickets']
    LOOP
        EXECUTE format($q$
            INSERT INTO crm_external_refs
                (source_service, source_type, source_id, target_type, target_id, origin)
            SELECT 'crm-legacy-code', %L, id, %L, crm_id_to_uuid(id)::text, 'platform'
              FROM %I
             WHERE id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
            ON CONFLICT (source_service, source_type, source_id) DO NOTHING
        $q$, t, t, t);
    END LOOP;
END
$legacy$;

-- ---- drop the foreign keys ------------------------------------------------
ALTER TABLE crm_activities          DROP CONSTRAINT crm_activities_account_id_fkey;
ALTER TABLE crm_activities          DROP CONSTRAINT crm_activities_contact_id_fkey;
ALTER TABLE crm_activities          DROP CONSTRAINT crm_activities_deal_id_fkey;
ALTER TABLE crm_buying_signals      DROP CONSTRAINT crm_buying_signals_account_id_fkey;
ALTER TABLE crm_contacts            DROP CONSTRAINT crm_contacts_account_id_fkey;
ALTER TABLE crm_deals               DROP CONSTRAINT crm_deals_account_id_fkey;
ALTER TABLE crm_journey_enrollments DROP CONSTRAINT crm_journey_enrollments_journey_id_fkey;
ALTER TABLE crm_journey_step_logs   DROP CONSTRAINT crm_journey_step_logs_enrollment_id_fkey;
ALTER TABLE crm_journey_steps       DROP CONSTRAINT crm_journey_steps_journey_id_fkey;
ALTER TABLE crm_loyalty_outlets     DROP CONSTRAINT crm_loyalty_outlets_account_id_fkey;
ALTER TABLE crm_outlets             DROP CONSTRAINT crm_outlets_account_id_fkey;
ALTER TABLE crm_quotes              DROP CONSTRAINT crm_quotes_account_id_fkey;
ALTER TABLE crm_quotes              DROP CONSTRAINT crm_quotes_deal_id_fkey;
ALTER TABLE crm_tickets             DROP CONSTRAINT crm_tickets_account_id_fkey;
ALTER TABLE crm_tickets             DROP CONSTRAINT crm_tickets_contact_id_fkey;
ALTER TABLE crm_tickets             DROP CONSTRAINT crm_tickets_deal_id_fkey;

-- ---- primary keys ---------------------------------------------------------
ALTER TABLE crm_accounts               ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_activities             ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_audit_entries          ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_bridge_field_mappings  ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_bridge_pending_imports ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_bridge_streams         ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_bridge_sync_log        ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_budget_plans           ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_buying_signals         ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_campaigns              ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_contacts               ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_content_assets         ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_deals                  ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_email_sends            ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_events                 ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_export_customers       ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_export_jobs            ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_journey_enrollments    ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_journey_steps          ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_journeys               ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_leads                  ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_loyalty_outlets        ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_loyalty_promotions     ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_loyalty_tiers          ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_module_records         ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_mqls                   ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_outlets                ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_personas               ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_quotes                 ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_segments               ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_seo_audit_jobs         ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_seo_keywords           ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_social_posts           ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);
ALTER TABLE crm_tickets                ALTER COLUMN id TYPE UUID USING crm_id_to_uuid(id);

-- ---- referencing columns --------------------------------------------------
ALTER TABLE crm_activities            ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_activities            ALTER COLUMN contact_id    TYPE UUID USING crm_id_to_uuid(contact_id);
ALTER TABLE crm_activities            ALTER COLUMN deal_id       TYPE UUID USING crm_id_to_uuid(deal_id);
ALTER TABLE crm_buying_signals        ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_contacts              ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_content_assets        ALTER COLUMN campaign_id   TYPE UUID USING crm_id_to_uuid(campaign_id);
ALTER TABLE crm_deals                 ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_email_sends           ALTER COLUMN segment_id    TYPE UUID USING crm_id_to_uuid(segment_id);
ALTER TABLE crm_journey_enrollments   ALTER COLUMN contact_id    TYPE UUID USING crm_id_to_uuid(contact_id);
ALTER TABLE crm_journey_enrollments   ALTER COLUMN journey_id    TYPE UUID USING crm_id_to_uuid(journey_id);
ALTER TABLE crm_journey_enrollments   ALTER COLUMN lead_id       TYPE UUID USING crm_id_to_uuid(lead_id);
ALTER TABLE crm_journey_step_logs     ALTER COLUMN enrollment_id TYPE UUID USING crm_id_to_uuid(enrollment_id);
ALTER TABLE crm_journey_step_logs     ALTER COLUMN step_id       TYPE UUID USING crm_id_to_uuid(step_id);
ALTER TABLE crm_journey_steps         ALTER COLUMN journey_id    TYPE UUID USING crm_id_to_uuid(journey_id);
ALTER TABLE crm_leads                 ALTER COLUMN converted_deal_id TYPE UUID USING crm_id_to_uuid(converted_deal_id);
ALTER TABLE crm_loyalty_outlets       ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_loyalty_outlets       ALTER COLUMN tier_id       TYPE UUID USING crm_id_to_uuid(tier_id);
ALTER TABLE crm_loyalty_promotions    ALTER COLUMN tier_id       TYPE UUID USING crm_id_to_uuid(tier_id);
ALTER TABLE crm_mqls                  ALTER COLUMN campaign_id   TYPE UUID USING crm_id_to_uuid(campaign_id);
ALTER TABLE crm_mqls                  ALTER COLUMN lead_id       TYPE UUID USING crm_id_to_uuid(lead_id);
ALTER TABLE crm_outlets               ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_personas              ALTER COLUMN journey_id    TYPE UUID USING crm_id_to_uuid(journey_id);
ALTER TABLE crm_personas              ALTER COLUMN segment_id    TYPE UUID USING crm_id_to_uuid(segment_id);
ALTER TABLE crm_quotes                ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_quotes                ALTER COLUMN deal_id       TYPE UUID USING crm_id_to_uuid(deal_id);
ALTER TABLE crm_social_posts          ALTER COLUMN campaign_id   TYPE UUID USING crm_id_to_uuid(campaign_id);
ALTER TABLE crm_tickets               ALTER COLUMN account_id    TYPE UUID USING crm_id_to_uuid(account_id);
ALTER TABLE crm_tickets               ALTER COLUMN contact_id    TYPE UUID USING crm_id_to_uuid(contact_id);
ALTER TABLE crm_tickets               ALTER COLUMN deal_id       TYPE UUID USING crm_id_to_uuid(deal_id);
ALTER TABLE crm_bridge_field_mappings ALTER COLUMN stream_id     TYPE UUID USING crm_id_to_uuid(stream_id);
ALTER TABLE crm_bridge_sync_log       ALTER COLUMN stream_id     TYPE UUID USING crm_id_to_uuid(stream_id);

-- ---- restore the foreign keys ---------------------------------------------
ALTER TABLE crm_activities          ADD CONSTRAINT crm_activities_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_activities          ADD CONSTRAINT crm_activities_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES crm_contacts(id) ON DELETE SET NULL;
ALTER TABLE crm_activities          ADD CONSTRAINT crm_activities_deal_id_fkey FOREIGN KEY (deal_id) REFERENCES crm_deals(id) ON DELETE SET NULL;
ALTER TABLE crm_buying_signals      ADD CONSTRAINT crm_buying_signals_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_contacts            ADD CONSTRAINT crm_contacts_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_deals               ADD CONSTRAINT crm_deals_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_journey_enrollments ADD CONSTRAINT crm_journey_enrollments_journey_id_fkey FOREIGN KEY (journey_id) REFERENCES crm_journeys(id) ON DELETE CASCADE;
ALTER TABLE crm_journey_step_logs   ADD CONSTRAINT crm_journey_step_logs_enrollment_id_fkey FOREIGN KEY (enrollment_id) REFERENCES crm_journey_enrollments(id) ON DELETE CASCADE;
ALTER TABLE crm_journey_steps       ADD CONSTRAINT crm_journey_steps_journey_id_fkey FOREIGN KEY (journey_id) REFERENCES crm_journeys(id) ON DELETE CASCADE;
ALTER TABLE crm_loyalty_outlets     ADD CONSTRAINT crm_loyalty_outlets_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_outlets             ADD CONSTRAINT crm_outlets_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_quotes              ADD CONSTRAINT crm_quotes_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_quotes              ADD CONSTRAINT crm_quotes_deal_id_fkey FOREIGN KEY (deal_id) REFERENCES crm_deals(id) ON DELETE SET NULL;
ALTER TABLE crm_tickets             ADD CONSTRAINT crm_tickets_account_id_fkey FOREIGN KEY (account_id) REFERENCES crm_accounts(id) ON DELETE SET NULL;
ALTER TABLE crm_tickets             ADD CONSTRAINT crm_tickets_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES crm_contacts(id) ON DELETE SET NULL;
ALTER TABLE crm_tickets             ADD CONSTRAINT crm_tickets_deal_id_fkey FOREIGN KEY (deal_id) REFERENCES crm_deals(id) ON DELETE SET NULL;

-- ---- the database mints ids from here on ----------------------------------
ALTER TABLE crm_accounts               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_activities             ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_audit_entries          ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_bridge_field_mappings  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_bridge_pending_imports ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_bridge_streams         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_bridge_sync_log        ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_budget_plans           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_buying_signals         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_campaigns              ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_contacts               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_content_assets         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_deals                  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_email_sends            ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_events                 ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_export_customers       ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_export_jobs            ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_journey_enrollments    ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_journey_steps          ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_journeys               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_leads                  ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_loyalty_outlets        ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_loyalty_promotions     ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_loyalty_tiers          ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_module_records         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_mqls                   ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_outlets                ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_personas               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_quotes                 ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_segments               ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_seo_audit_jobs         ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_seo_keywords           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_social_posts           ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE crm_tickets                ALTER COLUMN id SET DEFAULT gen_random_uuid();

-- The counter table has no purpose once ids are uuid. Repository.NextID and its
-- callers go with it.
DROP TABLE IF EXISTS crm_id_counters;

DROP FUNCTION crm_id_to_uuid(TEXT);
