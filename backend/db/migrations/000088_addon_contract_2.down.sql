-- 000088_addon_contract_2, down. account_uid was written beside account_id
-- the whole time, so dropping account_id loses nothing a contract-1 backend
-- reads.
ALTER TABLE targets DROP COLUMN IF EXISTS field_roles;
DROP INDEX IF EXISTS idx_target_account_bindings_account_id;
ALTER TABLE target_binding_conflicts DROP CONSTRAINT IF EXISTS target_binding_conflicts_account_id_check;
ALTER TABLE target_account_bindings DROP CONSTRAINT IF EXISTS target_account_bindings_account_id_check;
ALTER TABLE target_binding_conflicts DROP COLUMN IF EXISTS account_id;
ALTER TABLE target_account_bindings DROP COLUMN IF EXISTS account_id;
