ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_pending_unresolved;
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_fields_not_blank;
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_duration_positive;
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_status_enum;

ALTER TABLE direct_role_grants DROP CONSTRAINT IF EXISTS ck_direct_role_grants_fields_not_blank;

ALTER TABLE claim_profiles DROP CONSTRAINT IF EXISTS ck_claim_profiles_format_type;

ALTER TABLE mapping_rules DROP CONSTRAINT IF EXISTS ck_mapping_rules_not_self_edge;
ALTER TABLE mapping_rules DROP CONSTRAINT IF EXISTS ck_mapping_rules_target_not_blank;
ALTER TABLE mapping_rules DROP CONSTRAINT IF EXISTS ck_mapping_rules_source_not_blank;
ALTER TABLE mapping_rules DROP CONSTRAINT IF EXISTS ck_mapping_rules_version_positive;
