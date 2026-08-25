-- 000032_target_account_bindings.down.sql
--
-- Dropping this loses which account belonged to whom, and the add-on's own
-- store is the only remaining copy. That is survivable — it is the store the
-- apply actually consults — but the inventory would then report every managed
-- account as unmanaged until the next apply re-records it, so a rollback here
-- is a rollback of the surfaces built on it, not only of the table.

DROP INDEX IF EXISTS idx_target_account_bindings_uid;
DROP INDEX IF EXISTS idx_target_account_bindings_username;
DROP TABLE IF EXISTS target_account_bindings;
