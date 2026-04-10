-- Contract hardening: database-level invariants for mission-critical workflows.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_mapping_rules_version_positive') THEN
        ALTER TABLE mapping_rules
        ADD CONSTRAINT ck_mapping_rules_version_positive CHECK (version > 0);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_mapping_rules_source_not_blank') THEN
        ALTER TABLE mapping_rules
        ADD CONSTRAINT ck_mapping_rules_source_not_blank
        CHECK (btrim(source_zitadel_project_id) <> '' AND btrim(source_zitadel_role_key) <> '');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_mapping_rules_target_not_blank') THEN
        ALTER TABLE mapping_rules
        ADD CONSTRAINT ck_mapping_rules_target_not_blank
        CHECK (btrim(target_zitadel_project_id) <> '' AND btrim(target_zitadel_role_key) <> '');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_mapping_rules_not_self_edge') THEN
        ALTER TABLE mapping_rules
        ADD CONSTRAINT ck_mapping_rules_not_self_edge
        CHECK (
            source_zitadel_project_id <> target_zitadel_project_id
            OR source_zitadel_role_key <> target_zitadel_role_key
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_claim_profiles_format_type') THEN
        ALTER TABLE claim_profiles
        ADD CONSTRAINT ck_claim_profiles_format_type
        CHECK (format_type IN ('array', 'csv', 'space_delimited'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_direct_role_grants_fields_not_blank') THEN
        ALTER TABLE direct_role_grants
        ADD CONSTRAINT ck_direct_role_grants_fields_not_blank
        CHECK (
            btrim(user_id) <> ''
            AND btrim(zitadel_project_id) <> ''
            AND btrim(zitadel_role_key) <> ''
            AND btrim(granted_by) <> ''
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_access_requests_status_enum') THEN
        ALTER TABLE access_requests
        ADD CONSTRAINT ck_access_requests_status_enum
        CHECK (status IN ('pending', 'approved', 'rejected'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_access_requests_duration_positive') THEN
        ALTER TABLE access_requests
        ADD CONSTRAINT ck_access_requests_duration_positive
        CHECK (duration_days IS NULL OR duration_days > 0);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_access_requests_fields_not_blank') THEN
        ALTER TABLE access_requests
        ADD CONSTRAINT ck_access_requests_fields_not_blank
        CHECK (
            btrim(requester_user_id) <> ''
            AND btrim(zitadel_project_id) <> ''
            AND btrim(zitadel_role_key) <> ''
            AND btrim(justification) <> ''
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_access_requests_pending_unresolved') THEN
        ALTER TABLE access_requests
        ADD CONSTRAINT ck_access_requests_pending_unresolved
        CHECK (
            (status = 'pending' AND reviewer_user_id IS NULL AND resolved_at IS NULL)
            OR (status IN ('approved', 'rejected') AND reviewer_user_id IS NOT NULL AND resolved_at IS NOT NULL)
        );
    END IF;
END $$;
