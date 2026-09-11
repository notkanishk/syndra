-- Rolling back must not destroy or relabel findings the sweep closed as gone.
-- NOT VALID leaves existing 'resolved' rows in place; only new rows are checked.
ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_status_check;
ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_status_check
    CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external')) NOT VALID;
