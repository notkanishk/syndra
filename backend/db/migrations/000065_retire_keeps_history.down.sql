-- Rolling back means retired things are deleted after all: the unique name and
-- rule logic can only be restored once they are gone.
DELETE FROM mapping_rules WHERE retired_at IS NOT NULL;

DELETE FROM bundle_version_includes
 WHERE child_bundle_id IN (SELECT id FROM bundles WHERE retired_at IS NOT NULL);
DELETE FROM bundle_includes
 WHERE child_bundle_id IN (SELECT id FROM bundles WHERE retired_at IS NOT NULL);
DELETE FROM bundles WHERE retired_at IS NOT NULL;

DROP INDEX idx_mapping_rules_logic;
CREATE UNIQUE INDEX idx_mapping_rules_logic ON mapping_rules (
    source_zitadel_project_id, source_zitadel_role_key,
    target_zitadel_project_id, target_zitadel_role_key
);

DROP INDEX bundles_name_key;
ALTER TABLE bundles ADD CONSTRAINT bundles_name_key UNIQUE (name);

ALTER TABLE mapping_rules DROP COLUMN retired_by, DROP COLUMN retired_at;
ALTER TABLE bundles DROP COLUMN retired_by, DROP COLUMN retired_at;
