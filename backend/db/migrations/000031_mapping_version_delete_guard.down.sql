-- 000031_mapping_version_delete_guard.down.sql
-- Back to the UPDATE-only guard 000029 installed, and no guard at all on the
-- entries.

DROP TRIGGER IF EXISTS target_mapping_version_entries_immutable ON target_mapping_version_entries;
DROP FUNCTION IF EXISTS reject_target_mapping_version_entry_mutation();

DROP TRIGGER IF EXISTS target_mapping_versions_immutable ON target_mapping_versions;
CREATE TRIGGER target_mapping_versions_immutable
    BEFORE UPDATE ON target_mapping_versions
    FOR EACH ROW EXECUTE FUNCTION reject_target_mapping_version_mutation();
