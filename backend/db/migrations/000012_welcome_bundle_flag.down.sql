DROP INDEX IF EXISTS idx_bundles_welcome_unique;
ALTER TABLE bundles DROP COLUMN IF EXISTS is_welcome;
