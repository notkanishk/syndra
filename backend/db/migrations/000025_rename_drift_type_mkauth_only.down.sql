-- 000025_rename_drift_type_mkauth_only.down.sql
-- Reverse of the up: put the pre-rename value and constraint back.

ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_drift_type_check;

UPDATE drift_items SET drift_type = 'mkauth_only' WHERE drift_type = 'syndra_only';

ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_drift_type_check
    CHECK (drift_type IN ('zitadel_only', 'mkauth_only'));
