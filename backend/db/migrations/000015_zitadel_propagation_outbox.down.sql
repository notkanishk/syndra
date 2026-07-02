-- 000015_zitadel_propagation_outbox.down.sql
-- Reverses 000015. Outbox rows are workflow state, not historical record;
-- dropping the table loses only un-drained buffer entries (re-creatable by the
-- operator). source/source_ref backfill to 'direct'/NULL on restore is lossless
-- for sub-phase-1 data (all rows are source='direct' until sub-phase 3 ships).

ALTER TABLE direct_role_grants DROP COLUMN IF EXISTS source_ref;
ALTER TABLE direct_role_grants DROP COLUMN IF EXISTS source;
DROP TABLE IF EXISTS pending_zitadel_propagations;
