ALTER TABLE user_bundle_assignments
    DROP COLUMN IF EXISTS moved_from_version,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS assigned_by;
