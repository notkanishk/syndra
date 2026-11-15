-- 000098_mirror_is_not_an_identity (addon-contract 7t-6)
--
-- target_account_bindings is the backend's MIRROR of the add-on's bindings
-- (design §9.4b, ADR-007): which subject an account belongs to is the add-on's
-- to say, and account_id is a key within one read, not an identity across
-- reads. The unique indexes on (target, username), (target, account_uid) and
-- (target, account_id) made the mirror an identity anyway: when TrueNAS hands a
-- freed uid (or a freed name) to a new account and the add-on rightly binds
-- person B to it, A's old mirror row still holds that number, the insert
-- violated the index, and B's correct apply settled as a conflict ("Do not
-- retry").
--
-- So the three go. One subject, one mirror row stays (the primary key,
-- (target, subject_id)). On a write the add-on's answer is the authority:
-- db.RecordTargetBinding clears another subject's row on the same account (the
-- same id where both rows carry one, the name only where either has none) in
-- the same transaction, logged and audited (binding.<target>.stale_cleared),
-- instead of refusing.
--
-- No lookup index replaces them: the only read by name or number is that
-- clear, scoped to one target, over a few hundred rows that the primary key's
-- leading `target` column already narrows to.
--
-- target_binding_conflicts is no longer written or read (its only writer was
-- the refusal above). It is kept, untouched, for one release so a rollback to
-- the contract-1 backend (tasks 1.10: migrations are never run down there)
-- still finds the table it reads; it is dropped with account_uid (task 1.7b).
DROP INDEX IF EXISTS idx_target_account_bindings_username;
DROP INDEX IF EXISTS idx_target_account_bindings_uid;
DROP INDEX IF EXISTS idx_target_account_bindings_account_id;
