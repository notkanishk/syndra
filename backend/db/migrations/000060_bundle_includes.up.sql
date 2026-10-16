-- Included bundles (reconcile B8, owner's pick 2e, 9 Oct 2026). A bundle can
-- include another bundle's pinned version, one level deep. The parent
-- version's effective roles are its own plus the pinned child version's. A
-- role present both ways is held once and survives either being removed.
--
-- The working copy, like bundle_roles: edits here reach nobody until the
-- parent publishes.
CREATE TABLE bundle_includes (
    parent_bundle_id UUID NOT NULL REFERENCES bundles(id) ON DELETE CASCADE,
    child_bundle_id  UUID NOT NULL REFERENCES bundles(id) ON DELETE RESTRICT,
    child_version_id UUID NOT NULL REFERENCES bundle_versions(id) ON DELETE RESTRICT,
    PRIMARY KEY (parent_bundle_id, child_bundle_id),
    CHECK (parent_bundle_id <> child_bundle_id)
);
CREATE INDEX idx_bundle_includes_child ON bundle_includes (child_bundle_id);

-- The published snapshot, like bundle_version_roles: what a version included,
-- pinned. RESTRICT on the child side means no delete can silently strip the
-- roles a parent's holders get through it; retiring an included bundle is
-- refused by name before it gets here.
CREATE TABLE bundle_version_includes (
    version_id       UUID NOT NULL REFERENCES bundle_versions(id) ON DELETE CASCADE,
    child_bundle_id  UUID NOT NULL REFERENCES bundles(id) ON DELETE RESTRICT,
    child_version_id UUID NOT NULL REFERENCES bundle_versions(id) ON DELETE RESTRICT,
    PRIMARY KEY (version_id, child_bundle_id)
);
CREATE INDEX idx_bundle_version_includes_child ON bundle_version_includes (child_bundle_id);

-- What a version gives, in one place: its own roles, and each included
-- version's roles with the bundle they came through. Every read that decides
-- access reads this, so closures, the claims cache, drift and the person page
-- all agree on what an include means.
CREATE VIEW bundle_version_effective_roles AS
    SELECT r.version_id, r.zitadel_project_id, r.zitadel_role_key, NULL::uuid AS via_bundle_id
      FROM bundle_version_roles r
    UNION ALL
    SELECT i.version_id, cr.zitadel_project_id, cr.zitadel_role_key, i.child_bundle_id
      FROM bundle_version_includes i
      JOIN bundle_version_roles cr ON cr.version_id = i.child_version_id;
