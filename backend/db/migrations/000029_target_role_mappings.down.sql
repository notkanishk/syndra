-- 000029_target_role_mappings.down.sql
--
-- Dropping the mappings drops what every add-on entitlement is derived from, so
-- the rollback refuses while any subject's access depends on one. There is no
-- reinterpretation available here as there is for a target column: a mapping
-- deleted is a role that silently stops reaching anything, and the resolver
-- would compute an empty entitlement set and converge every holder to nothing.
DO $$
DECLARE
    bound BIGINT;
BEGIN
    SELECT COUNT(*) INTO bound FROM target_role_mappings;
    IF bound > 0 THEN
        RAISE EXCEPTION
            'refusing to roll back: % role mapping(s) exist, and dropping them resolves every holder to no entitlement rather than to no change. Remove them through the mapping surface, which plans the consequence first.',
            bound;
    END IF;
END
$$;

DROP TRIGGER IF EXISTS target_mapping_versions_immutable ON target_mapping_versions;
DROP FUNCTION IF EXISTS reject_target_mapping_version_mutation();
DROP TABLE IF EXISTS target_mapping_version_entries;
DROP TABLE IF EXISTS target_mapping_versions;
DROP TABLE IF EXISTS target_role_mappings;
