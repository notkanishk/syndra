-- Migration 006: Governance integrity constraints
-- Adds defense-in-depth DB constraints for the most common injection points.
-- Most access_request and mapping_rule constraints are already covered in 004.

-- Bundles: name must not be blank (handler trims; this is defense-in-depth)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_bundles_name_not_blank') THEN
        ALTER TABLE bundles
            ADD CONSTRAINT ck_bundles_name_not_blank
            CHECK (btrim(name) <> '');
    END IF;
END $$;

-- Bundle roles: project and role identifiers must not be blank
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_bundle_roles_not_blank') THEN
        ALTER TABLE bundle_roles
            ADD CONSTRAINT ck_bundle_roles_not_blank
            CHECK (btrim(zitadel_project_id) <> '' AND btrim(zitadel_role_key) <> '');
    END IF;
END $$;

-- Direct grants: expires_at, when set, must be after created_at
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_direct_role_grants_expiry_after_create') THEN
        ALTER TABLE direct_role_grants
            ADD CONSTRAINT ck_direct_role_grants_expiry_after_create
            CHECK (expires_at IS NULL OR expires_at > created_at);
    END IF;
END $$;
