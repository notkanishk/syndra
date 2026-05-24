-- Reverts 000013. Rows with status='dropped_enrichment_incomplete' or empty
-- source_project must be removed before the original checks can be restored
-- — otherwise the ADD CONSTRAINT will fail validation against existing data.

DELETE FROM webhook_events
    WHERE status = 'dropped_enrichment_incomplete'
       OR btrim(source_project) = '';

ALTER TABLE webhook_events
    DROP CONSTRAINT IF EXISTS webhook_events_source_project_check;

ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_source_project_check
        CHECK (btrim(source_project) <> '');

ALTER TABLE webhook_events
    DROP CONSTRAINT IF EXISTS webhook_events_status_check;

ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_status_check
        CHECK (status IN ('received', 'processed', 'failed'));
