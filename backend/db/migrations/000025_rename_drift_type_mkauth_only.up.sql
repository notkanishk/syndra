-- 000025_rename_drift_type_mkauth_only.up.sql
-- The application was renamed MkAuth -> Syndra. Every other occurrence was a
-- text replacement, but drift_items.drift_type is stored data constrained by a
-- CHECK, so the rename has to move the rows and the constraint together.
--
-- Order matters: the constraint must come off before the UPDATE, because the
-- old constraint forbids the new value and the new one forbids the old rows.

ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_drift_type_check;

UPDATE drift_items SET drift_type = 'syndra_only' WHERE drift_type = 'mkauth_only';

ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_drift_type_check
    CHECK (drift_type IN ('zitadel_only', 'syndra_only'));
