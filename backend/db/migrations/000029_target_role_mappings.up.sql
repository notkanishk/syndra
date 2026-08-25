-- 000029_target_role_mappings.up.sql
-- Add-on platform, group 7: what a Zitadel role means on a target (change
-- `addon-platform`, design §6).
--
-- Without this table the resolver has no source for the role-derived half of an
-- entitlement set. `role_key='maker'` means nothing to TrueNAS until somebody
-- says it means `group=lab_makers`, and that sentence is a policy decision with
-- the same blast radius as a bundle edit: changing it silently changes what
-- every holder of that role can reach. So it carries the same versioning,
-- rollback and audit that bundles carry, and for the same reason.
--
-- It is deliberately NOT deployment configuration. A mapping in an env var is a
-- mapping with no history, no actor, and no plan before it lands.

-- The working copy. Edits land here and reach nobody until they are resolved
-- through a cascade, exactly as `bundle_roles` does.
--
-- `field` and `value` are the add-on's own vocabulary — Syndra fills them and
-- never learns what `lab_makers` means. Validation is split for that reason:
-- Syndra checks the field is in the add-on's declared schema and the role
-- exists, and the add-on confirms the value resolves on its target.
CREATE TABLE IF NOT EXISTS target_role_mappings (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target     TEXT NOT NULL REFERENCES targets(target),
    project_id VARCHAR(255) NOT NULL,
    role_key   VARCHAR(255) NOT NULL,
    field      VARCHAR(255) NOT NULL,
    value      TEXT NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One binding per (target, project, role, field). Two rows binding the same
    -- role's `group` to different values is not a richer mapping, it is a
    -- resolver that returns whichever the database happened to order first —
    -- and a subject whose access depends on that ordering.
    --
    -- Multi-valued fields (`group[]`) are expressed as distinct FIELDS on the
    -- add-on's schema, not as duplicate rows here, because the add-on is the
    -- only thing that knows which of its fields take a list.
    UNIQUE (target, project_id, role_key, field)
);

CREATE INDEX IF NOT EXISTS idx_target_role_mappings_role
    ON target_role_mappings (target, project_id, role_key);

-- The reverse direction, which the lifecycle trigger needs: "which targets does
-- this role reach", asked on every grant change.
CREATE INDEX IF NOT EXISTS idx_target_role_mappings_lookup
    ON target_role_mappings (project_id, role_key);

-- Published snapshots of the whole mapping set for a target. Immutable once
-- written.
--
-- Versioned per TARGET rather than per row. A mapping edit is only meaningful
-- against the set it sits in — "the TrueNAS mapping as it stood on Tuesday" is
-- the thing an operator wants to roll back to, and a per-row version would let
-- a rollback restore one binding into a set the rest of which has moved on.
CREATE TABLE IF NOT EXISTS target_mapping_versions (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target       TEXT NOT NULL REFERENCES targets(target),
    version      INT NOT NULL,
    note         TEXT NOT NULL DEFAULT '',
    published_by VARCHAR(255) NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (target, version)
);

CREATE INDEX IF NOT EXISTS idx_target_mapping_versions_target
    ON target_mapping_versions (target, version DESC);

CREATE TABLE IF NOT EXISTS target_mapping_version_entries (
    version_id UUID NOT NULL REFERENCES target_mapping_versions(id) ON DELETE CASCADE,
    project_id VARCHAR(255) NOT NULL,
    role_key   VARCHAR(255) NOT NULL,
    field      VARCHAR(255) NOT NULL,
    value      TEXT NOT NULL,
    PRIMARY KEY (version_id, project_id, role_key, field)
);

-- A published version is a historical record and must not be editable, for the
-- same reason a desired-state snapshot must not be: a rollback compares against
-- it, and a record that can be edited is a comparison against nothing.
CREATE OR REPLACE FUNCTION reject_target_mapping_version_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'target_mapping_versions rows are immutable history (attempted %)', TG_OP;
END;
$$;

DROP TRIGGER IF EXISTS target_mapping_versions_immutable ON target_mapping_versions;
CREATE TRIGGER target_mapping_versions_immutable
    BEFORE UPDATE ON target_mapping_versions
    FOR EACH ROW EXECUTE FUNCTION reject_target_mapping_version_mutation();
