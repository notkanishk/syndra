-- Permit the 'dropped_enrichment_incomplete' webhook_events status (audit refs
-- C11, D8) so Zitadel-shape grant events whose enrichment cannot resolve
-- source_project / role_keys can be persisted instead of silently swallowed
-- behind the storm-prevention 200-ack.
--
-- The original status check (000007) restricted status to
-- (received, processed, failed); the original source_project check required
-- a non-empty value. A dropped enrichment-incomplete row has neither a
-- resolved source_project nor a processed/failed lifecycle — relax both
-- checks for that one status only.

ALTER TABLE webhook_events
    DROP CONSTRAINT IF EXISTS webhook_events_status_check;

ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_status_check
        CHECK (status IN ('received', 'processed', 'failed', 'dropped_enrichment_incomplete'));

ALTER TABLE webhook_events
    DROP CONSTRAINT IF EXISTS webhook_events_source_project_check;

ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_source_project_check
        CHECK (status = 'dropped_enrichment_incomplete' OR btrim(source_project) <> '');
