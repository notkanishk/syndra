DROP TABLE IF EXISTS config_settings;
DROP INDEX IF EXISTS idx_pending_zitadel_propagations_source;
ALTER TABLE pending_zitadel_propagations DROP COLUMN IF EXISTS source_ref;
ALTER TABLE pending_zitadel_propagations DROP COLUMN IF EXISTS source;
ALTER TABLE bundles       DROP COLUMN IF EXISTS confirmation_mode;
ALTER TABLE mapping_rules DROP COLUMN IF EXISTS confirmation_mode;
