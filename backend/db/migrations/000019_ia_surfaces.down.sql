-- 000019_ia_surfaces.down.sql

DROP INDEX IF EXISTS idx_pending_zitadel_propagations_cascade;

ALTER TABLE pending_zitadel_propagations
    DROP COLUMN IF EXISTS cascade_id;

ALTER TABLE drift_items
    DROP COLUMN IF EXISTS upstream_actor,
    DROP COLUMN IF EXISTS upstream_created_at,
    DROP COLUMN IF EXISTS last_seen_at;
