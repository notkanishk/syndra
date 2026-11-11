-- 000088_addon_contract_2
--
-- Add-on contract 2 (openspec/changes/addon-contract, tasks 1.3 and 1.7).
--
-- 1. account_id: the target's own stable id for an account, as contract 2
--    names it (a string: a TrueNAS uid, a UniFi or Ruby UUID; decision 26).
--    It sits BESIDE account_uid in both tables, never instead of it, and the
--    backend writes both for one release (decision 10), so a rollback to the
--    contract-1 backend still reads every row by account_uid. account_uid is
--    dropped a release later by its own task (1.7b).
ALTER TABLE target_account_bindings ADD COLUMN IF NOT EXISTS account_id TEXT;
ALTER TABLE target_binding_conflicts ADD COLUMN IF NOT EXISTS account_id TEXT;

ALTER TABLE target_account_bindings DROP CONSTRAINT IF EXISTS target_account_bindings_account_id_check;
ALTER TABLE target_account_bindings ADD CONSTRAINT target_account_bindings_account_id_check
    CHECK (account_id IS NULL OR btrim(account_id) <> '');
ALTER TABLE target_binding_conflicts DROP CONSTRAINT IF EXISTS target_binding_conflicts_account_id_check;
ALTER TABLE target_binding_conflicts ADD CONSTRAINT target_binding_conflicts_account_id_check
    CHECK (account_id IS NULL OR btrim(account_id) <> '');

-- The backfill, re-runnable: the backend runs the same statements again at
-- every start (db.BackfillAccountIDs), so a row a rolled-back contract-1
-- backend wrote or UPDATED meanwhile (account_uid alone, or a new uid for a
-- renamed or recreated account) is corrected on the next forward deploy. A
-- contract-2 write keeps account_uid equal to account_id whenever it writes
-- an id (NULL for a non-numeric one), so this never rewrites what it wrote.
UPDATE target_account_bindings SET account_id = account_uid::text
 WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text;
UPDATE target_binding_conflicts SET account_id = account_uid::text
 WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text;

-- One account, one subject, on the stable id, as on account_uid. On the
-- bindings table ONLY: target_binding_conflicts keeps resolved history, so a
-- unique (target, account_id) there would refuse the second conflict on an
-- account once the first was resolved. It stays unique only on its
-- open-conflict index (000039).
CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_account_id
    ON target_account_bindings(target, account_id)
    WHERE account_id IS NOT NULL;

-- 2. field_roles: each entitlement field's role as the add-on last declared it
--    and the backend accepted it (design §3), persisted at every registration
--    and refresh. Every reader of a field's meaning reads it from here rather
--    than from a live manifest, so revocation keeps working while the add-on
--    is down; a manifest that changes a field's role is refused and this map
--    is kept. NULL until the first contract-2 manifest is accepted.
ALTER TABLE targets ADD COLUMN IF NOT EXISTS field_roles JSONB;
