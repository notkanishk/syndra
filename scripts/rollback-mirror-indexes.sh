#!/usr/bin/env bash
# scripts/rollback-mirror-indexes.sh — put back the binding mirror's unique
# indexes before a backend older than migration 000098 runs, and again before
# any forward deploy that re-runs 000088 (addon-contract 7t-6).
#
# Why: 000098 dropped the unique indexes on target_account_bindings
# (target, username), (target, account_uid) and (target, account_id). A
# contract-1 backend reads a unique violation on them as "this account is
# somebody else's"; without them it silently records two people on one
# account, and its drift then matches both rows to that account. And the
# first forward deploy after the rollback re-runs 000088 from version 87,
# whose CREATE UNIQUE INDEX on (target, account_id) fails on any duplicate
# written meanwhile, leaving the migration dirty and the backend down.
#
# What it runs is 000098's down migration, byte for byte (a test in
# backend/internal/db holds the two equal): duplicates on each key resolved
# by keeping the row last seen (then bound_at, then subject_id), each cleared
# row audited as binding.<target>.stale_cleared by system:migration, then the
# three indexes created IF NOT EXISTS. Idempotent: a second run clears
# nothing and creates nothing. It does not touch schema_migrations.
#
# Usage, on the host, from any directory (copy it out of the checkout first:
# a rollback checks out a commit that does not have it):
#   scripts/rollback-mirror-indexes.sh            # dry run: counts, rehearses, rolls back
#   scripts/rollback-mirror-indexes.sh --apply    # one transaction, committed
#
# Env:
#   PG_CONTAINER  default syndra_postgres
#   PG_USER       default syndra
#   PG_DB         default syndradb

set -euo pipefail

APPLY=""
case "${1:-}" in
  "") ;;
  --apply) APPLY=1 ;;
  *) sed -n '2,31p' "$0" | sed 's|^# \{0,1\}||'; exit 2 ;;
esac

PG_CONTAINER="${PG_CONTAINER:-syndra_postgres}"
PG_USER="${PG_USER:-syndra}"
PG_DB="${PG_DB:-syndradb}"

# -i only where SQL is piped in; </dev/null everywhere else (reset-data.sh).
psql() { docker exec "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 "$@" </dev/null; }
psql_stdin() { docker exec -i "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 "$@"; }

read -r -d '' RESTORE <<'SQL' || true
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
SQL

echo "Binding mirror indexes — ${PG_CONTAINER}/${PG_DB}"
for key in username account_uid account_id; do
  n="$(psql -tAc "SELECT count(*) FROM (SELECT 1 FROM target_account_bindings WHERE ${key} IS NOT NULL
                  GROUP BY target, ${key} HAVING count(*) > 1) d")"
  echo "  duplicate ${key} groups: ${n}"
done
psql -tAc "SELECT '  present: ' || coalesce(string_agg(indexname, ', ' ORDER BY indexname), 'none')
             FROM pg_indexes WHERE tablename = 'target_account_bindings'
              AND indexname IN ('idx_target_account_bindings_username', 'idx_target_account_bindings_uid',
                                'idx_target_account_bindings_account_id')"

run() { { echo "BEGIN;"; printf '%s\n' "$RESTORE"; echo "$1;"; } | psql_stdin -q; }

if [[ -z "$APPLY" ]]; then
  run ROLLBACK >/dev/null
  echo "  Rehearsed in a transaction and rolled back: it runs clean. Pass --apply to commit."
  exit 0
fi
run COMMIT >/dev/null
echo "  Applied. Each cleared row is in audit_logs as binding.<target>.stale_cleared by system:migration."
