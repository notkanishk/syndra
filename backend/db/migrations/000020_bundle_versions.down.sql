-- Reverting drops the pins and the snapshots. The working copy (bundle_roles)
-- is untouched, so every holder falls back to "whatever the bundle holds now" —
-- which is exactly the pre-versioning behaviour, and is why the working copy
-- was never moved into the version tables.
--
-- Holders sitting on an older version LOSE that distinction on the way down:
-- there is nowhere to record it. That is a real data loss and the reason this
-- migration should not be reverted on a deployment where anyone has been left
-- behind deliberately.

DROP INDEX IF EXISTS idx_user_bundle_assignments_version;

ALTER TABLE user_bundle_assignments
    DROP COLUMN IF EXISTS version_id;

DROP TABLE IF EXISTS bundle_version_roles;
DROP INDEX IF EXISTS idx_bundle_versions_bundle;
DROP TABLE IF EXISTS bundle_versions;
