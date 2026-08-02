-- Bundle versioning.
--
-- A bundle edit used to reach every holder at once, which made editing a
-- bundle that fourteen people hold a decision nobody wanted to take casually.
-- Versions split the edit from its consequence: you change the bundle, then you
-- decide, separately and with the consequences in front of you, whether the
-- people already holding it move.
--
-- The model is git's, not a table of drafts:
--
--   bundle_roles          the WORKING COPY. Edits land here and reach nobody.
--   bundle_versions       published snapshots. Immutable once written.
--   bundle_version_roles  what each snapshot contained.
--
-- There is deliberately no draft table and no `is_draft` column. A draft is not
-- a state something is in — it is the difference between the working copy and
-- the latest published version, computed when asked. A stored draft flag would
-- be a second thing to keep true.
--
-- Every assignment pins a version. That pin is what a holder's access is
-- resolved through, so somebody left on v2 keeps exactly v2's roles after v3
-- publishes.

CREATE TABLE IF NOT EXISTS bundle_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    bundle_id UUID NOT NULL REFERENCES bundles(id) ON DELETE CASCADE,
    -- Per-bundle, starting at 1. Not global: "Lab Tech v2" is a sentence
    -- somebody says out loud, and a global sequence would make it meaningless.
    version INT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    published_by VARCHAR(255) NOT NULL DEFAULT '',
    published_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (bundle_id, version)
);

CREATE INDEX IF NOT EXISTS idx_bundle_versions_bundle ON bundle_versions(bundle_id, version DESC);

CREATE TABLE IF NOT EXISTS bundle_version_roles (
    version_id UUID NOT NULL REFERENCES bundle_versions(id) ON DELETE CASCADE,
    zitadel_project_id VARCHAR(255) NOT NULL,
    zitadel_role_key VARCHAR(255) NOT NULL,
    PRIMARY KEY (version_id, zitadel_project_id, zitadel_role_key)
);

-- Backfill: every bundle that exists today becomes v1, holding exactly what its
-- working copy holds. This is the honest reading — those roles are what its
-- holders have, so that is what v1 granted.
INSERT INTO bundle_versions (bundle_id, version, note, published_by)
SELECT id, 1, 'Existing bundle at the time versioning was introduced.', 'system'
FROM bundles
ON CONFLICT (bundle_id, version) DO NOTHING;

INSERT INTO bundle_version_roles (version_id, zitadel_project_id, zitadel_role_key)
SELECT bv.id, br.zitadel_project_id, br.zitadel_role_key
FROM bundle_roles br
JOIN bundle_versions bv ON bv.bundle_id = br.bundle_id AND bv.version = 1
ON CONFLICT DO NOTHING;

-- Pin every existing assignment to that v1. Added nullable, backfilled, then
-- made NOT NULL: an assignment with no version has no resolvable access, so the
-- column must never be optional once the data is in place.
ALTER TABLE user_bundle_assignments
    ADD COLUMN IF NOT EXISTS version_id UUID REFERENCES bundle_versions(id) ON DELETE CASCADE;

UPDATE user_bundle_assignments uba
SET version_id = bv.id
FROM bundle_versions bv
WHERE bv.bundle_id = uba.bundle_id AND bv.version = 1 AND uba.version_id IS NULL;

ALTER TABLE user_bundle_assignments
    ALTER COLUMN version_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_bundle_assignments_version
    ON user_bundle_assignments(version_id);
