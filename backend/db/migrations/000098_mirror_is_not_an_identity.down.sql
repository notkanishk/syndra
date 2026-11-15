-- 000098 down: the three unique indexes come back (000032, 000088).
--
-- Without them the mirror may hold two subjects on one name or number (a
-- contract-1 backend rolled back onto this schema writes without the clear),
-- and re-adding a unique index over such rows fails. So duplicates are
-- resolved first, deterministically, by the up's own rule: the row written
-- most recently is the add-on's latest answer and is kept (last_seen_at, then
-- bound_at, then subject_id as the tie-break that makes it a total order); every
-- other row on the same key is cleared, and each clear is audited as the up's
-- clear is (binding.<target>.stale_cleared, by system:migration). Refusing
-- instead would block a rollback in exactly the hour it is needed, over rows
-- the add-on's next answer re-records anyway.
--
-- Three keys, one after the other: a row kept on one key can still lose on
-- the next, and a cleared row is gone from the later passes. The name pass is
-- stricter than the up's clear (which keeps a same-name row on another id),
-- because the unique name index it restores allows no such pair.
WITH ranked AS (
    SELECT target, subject_id,
           ROW_NUMBER() OVER (PARTITION BY target, username
                              ORDER BY last_seen_at DESC, bound_at DESC, subject_id DESC) AS rank
      FROM target_account_bindings
), cleared AS (
    DELETE FROM target_account_bindings b USING ranked r
     WHERE b.target = r.target AND b.subject_id = r.subject_id AND r.rank > 1
    RETURNING b.target, b.subject_id, b.username, COALESCE(b.account_id, b.account_uid::text) AS account_id
)
INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details)
SELECT 'system:migration', subject_id, 'binding.' || target || '.stale_cleared', username,
       jsonb_build_object('username', username, 'account_id', account_id, 'why', 'migration 000098 down: same name')
  FROM cleared;

WITH ranked AS (
    SELECT target, subject_id,
           ROW_NUMBER() OVER (PARTITION BY target, account_uid
                              ORDER BY last_seen_at DESC, bound_at DESC, subject_id DESC) AS rank
      FROM target_account_bindings
     WHERE account_uid IS NOT NULL
), cleared AS (
    DELETE FROM target_account_bindings b USING ranked r
     WHERE b.target = r.target AND b.subject_id = r.subject_id AND r.rank > 1
    RETURNING b.target, b.subject_id, b.username, COALESCE(b.account_id, b.account_uid::text) AS account_id
)
INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details)
SELECT 'system:migration', subject_id, 'binding.' || target || '.stale_cleared', username,
       jsonb_build_object('username', username, 'account_id', account_id, 'why', 'migration 000098 down: same account_uid')
  FROM cleared;

WITH ranked AS (
    SELECT target, subject_id,
           ROW_NUMBER() OVER (PARTITION BY target, account_id
                              ORDER BY last_seen_at DESC, bound_at DESC, subject_id DESC) AS rank
      FROM target_account_bindings
     WHERE account_id IS NOT NULL
), cleared AS (
    DELETE FROM target_account_bindings b USING ranked r
     WHERE b.target = r.target AND b.subject_id = r.subject_id AND r.rank > 1
    RETURNING b.target, b.subject_id, b.username, b.account_id
)
INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details)
SELECT 'system:migration', subject_id, 'binding.' || target || '.stale_cleared', username,
       jsonb_build_object('username', username, 'account_id', account_id, 'why', 'migration 000098 down: same account_id')
  FROM cleared;

CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_username
    ON target_account_bindings(target, username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_uid
    ON target_account_bindings(target, account_uid)
    WHERE account_uid IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_target_account_bindings_account_id
    ON target_account_bindings(target, account_id)
    WHERE account_id IS NOT NULL;
