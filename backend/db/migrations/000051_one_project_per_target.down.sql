ALTER TABLE target_role_mappings DROP CONSTRAINT IF EXISTS target_role_mappings_target_project_fkey;
ALTER TABLE targets DROP CONSTRAINT IF EXISTS targets_target_project_key;
ALTER TABLE targets DROP CONSTRAINT IF EXISTS targets_project_id_key;
ALTER TABLE targets DROP COLUMN IF EXISTS project_id;
