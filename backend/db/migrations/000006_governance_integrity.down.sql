ALTER TABLE direct_role_grants DROP CONSTRAINT IF EXISTS ck_direct_role_grants_expiry_after_create;
ALTER TABLE bundle_roles DROP CONSTRAINT IF EXISTS ck_bundle_roles_not_blank;
ALTER TABLE bundles DROP CONSTRAINT IF EXISTS ck_bundles_name_not_blank;
