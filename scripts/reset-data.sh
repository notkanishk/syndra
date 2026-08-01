#!/usr/bin/env bash
# scripts/reset-data.sh — return a MkAuth deployment to a known starting state.
#
# There are two of those, and they are not the same thing:
#
#   demo  Delete only the rows that reference a demo fixture. Real people,
#         real projects and every decision an operator actually made survive.
#         This is what you want when a deployment was brought up with
#         MKAUTH_SEED_DEMO on and has since been used for real.
#
#   all   Truncate every operator-owned table. A genuine blank slate: no
#         bundles, no rules, no grants, no audit history. Schema and
#         migrations are untouched, so the backend comes back up and
#         re-registers nothing.
#
# Neither mode touches Zitadel. MkAuth's ledger is a record of what MkAuth
# decided — clearing it does not revoke anything upstream, and grants that
# exist in Zitadel without a local row will be re-detected by the next
# reconciliation sweep as unexplained access. That is the correct outcome:
# the sweep is how upstream state gets re-adopted deliberately rather than
# assumed.
#
# Usage:
#   scripts/reset-data.sh demo            # dry run: counts only, deletes nothing
#   scripts/reset-data.sh demo --apply
#   scripts/reset-data.sh all --apply
#
# Env:
#   PG_CONTAINER     default mkauth_postgres
#   REDIS_CONTAINER  default mkauth_redis
#   PG_USER          default mkauth
#   PG_DB            default mkauthdb

set -euo pipefail

MODE="${1:-}"
APPLY=""
[[ "${2:-}" == "--apply" ]] && APPLY=1

PG_CONTAINER="${PG_CONTAINER:-mkauth_postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER:-mkauth_redis}"
PG_USER="${PG_USER:-mkauth}"
PG_DB="${PG_DB:-mkauthdb}"

if [[ "$MODE" != "demo" && "$MODE" != "all" ]]; then
  sed -n '2,32p' "$0" | sed 's|^# \{0,1\}||'
  exit 2
fi

# Demo fixture ids. MUST stay identical to demo.ProjectIDs() and
# demo.UserIDs() in backend/internal/demo/catalog.go — a fixture added there
# and missed here would survive a reset and keep being served as real.
# backend/internal/demo/catalog_test.go asserts these two lists match.
DEMO_PROJECTS="'platform','printing','laser','doors','wiki','finance'"
DEMO_USERS="'dev_admin','sam_student','maya_staff','leo_mentor','ava_guest'"

psql() { docker exec -i "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 "$@"; }

# Operator-owned tables, ordered so children go before parents. Anything not
# listed here is either schema (schema_migrations) or a derived index the
# backend rebuilds on its own (zitadel_grants_index).
ALL_TABLES=(
  shadow_credential_audit shadow_credentials provisioning_intents
  webhook_events onboarding_triggers pending_zitadel_propagations
  drift_items external_grant_exclusions audit_logs access_requests
  user_bundle_assignments bundle_roles bundles mapping_rules
  direct_role_grants app_claim_overrides claim_profiles roles
  zitadel_grants_index config_settings
)

# Per-table WHERE clause selecting demo rows. Tables absent from this map hold
# nothing the seeder writes, so `demo` mode leaves them alone entirely.
demo_where() {
  case "$1" in
    bundle_roles)             echo "zitadel_project_id IN ($DEMO_PROJECTS)" ;;
    mapping_rules)            echo "source_zitadel_project_id IN ($DEMO_PROJECTS) OR target_zitadel_project_id IN ($DEMO_PROJECTS)" ;;
    claim_profiles)           echo "zitadel_project_id IN ($DEMO_PROJECTS)" ;;
    direct_role_grants)       echo "zitadel_project_id IN ($DEMO_PROJECTS) OR user_id IN ($DEMO_USERS)" ;;
    user_bundle_assignments)  echo "user_id IN ($DEMO_USERS)" ;;
    access_requests)          echo "requester_user_id IN ($DEMO_USERS) OR zitadel_project_id IN ($DEMO_PROJECTS)" ;;
    audit_logs)               echo "target_zitadel_user_id IN ($DEMO_USERS) OR actor_zitadel_user_id IN ($DEMO_USERS) OR action = 'cache.rebuilt'" ;;
    drift_items)              echo "project_id IN ($DEMO_PROJECTS) OR user_id IN ($DEMO_USERS)" ;;
    zitadel_grants_index)     echo "project_id IN ($DEMO_PROJECTS) OR user_id IN ($DEMO_USERS)" ;;
    *)                        echo "" ;;
  esac
}

# `bundles` has no demo column of its own — it is identified by the roles that
# point at it, so it is handled separately after bundle_roles is counted.
BUNDLE_PREDICATE="name IN ('Student Access','Staff Onboarding','Prototyping Mentor')"

echo "MkAuth reset — mode=${MODE} target=${PG_CONTAINER}/${PG_DB}"
echo

total=0
declare -a PLAN=()

if [[ "$MODE" == "demo" ]]; then
  for table in "${ALL_TABLES[@]}"; do
    where="$(demo_where "$table")"
    [[ -z "$where" ]] && continue
    n="$(psql -tAc "SELECT count(*) FROM ${table} WHERE ${where}")"
    (( n > 0 )) && { printf '  %-28s %5s rows\n' "$table" "$n"; PLAN+=("DELETE FROM ${table} WHERE ${where};"); total=$((total + n)); }
  done
  n="$(psql -tAc "SELECT count(*) FROM bundles WHERE ${BUNDLE_PREDICATE}")"
  (( n > 0 )) && { printf '  %-28s %5s rows\n' "bundles" "$n"; PLAN+=("DELETE FROM bundles WHERE ${BUNDLE_PREDICATE};"); total=$((total + n)); }
else
  for table in "${ALL_TABLES[@]}"; do
    n="$(psql -tAc "SELECT count(*) FROM ${table}")"
    (( n > 0 )) && printf '  %-28s %5s rows\n' "$table" "$n"
    total=$((total + n))
  done
  PLAN+=("TRUNCATE ${ALL_TABLES[*]} RESTART IDENTITY CASCADE;")
fi

if (( total == 0 )); then
  echo "  nothing to remove — already clean."
  exit 0
fi

echo
echo "  ${total} rows total."

# Demo and real data are not always cleanly separable, and pretending they are
# is how a reset quietly revokes somebody's access.
#
# A real person assigned to a demo bundle, or holding a direct grant on a demo
# project, loses that access when the fixture goes — user_bundle_assignments
# cascades on bundle delete, so those rows disappear without ever appearing in
# the per-table counts above. Naming them is the difference between a reset an
# operator chose and one they discover on Monday.
if [[ "$MODE" == "demo" ]]; then
  affected="$(psql -tAc "
    SELECT string_agg(DISTINCT user_id, ', ')
    FROM (
      SELECT a.user_id FROM user_bundle_assignments a
        JOIN bundles b ON b.id = a.bundle_id
       WHERE b.${BUNDLE_PREDICATE} AND a.user_id NOT IN ($DEMO_USERS)
      UNION
      SELECT user_id FROM direct_role_grants
       WHERE zitadel_project_id IN ($DEMO_PROJECTS) AND user_id NOT IN ($DEMO_USERS)
    ) t
  ")"
  if [[ -n "$affected" ]]; then
    echo
    echo "  WARNING — these are not demo accounts, and they lose access here:"
    echo "    ${affected}"
    echo "  They hold a demo bundle or a grant on a demo project. Removing the"
    echo "  fixture removes their access with it; MkAuth will not re-grant it."
    echo "  Re-grant from each person's page afterwards if it was deliberate."
  fi
fi

if [[ -z "$APPLY" ]]; then
  echo
  echo "Dry run. Nothing was deleted. Re-run with --apply to commit:"
  echo "  scripts/reset-data.sh ${MODE} --apply"
  exit 0
fi

echo
read -r -p "Type the mode name to confirm deletion (${MODE}): " answer
[[ "$answer" == "$MODE" ]] || { echo "Aborted — nothing deleted." >&2; exit 1; }

# One transaction. A partial reset that drops bundle_roles but keeps bundles
# leaves an empty bundle nobody can explain, which is worse than either
# end state.
{ echo "BEGIN;"; printf '%s\n' "${PLAN[@]}"; echo "COMMIT;"; } | psql -q
echo "Database reset."

# Redis holds only derived per-user claim caches (mapping:<user>:<project>),
# rebuilt on the next token issue or cache compile. Stale entries here would
# keep serving deleted grants until they expired.
if docker ps --format '{{.Names}}' | grep -qx "$REDIS_CONTAINER"; then
  docker exec "$REDIS_CONTAINER" redis-cli FLUSHDB >/dev/null
  echo "Claim cache flushed."
else
  echo "warning: ${REDIS_CONTAINER} not running — flush the claim cache before trusting any token." >&2
fi

echo
echo "Next:"
echo "  1. Confirm MKAUTH_SEED_DEMO=false in .env, or the next restart re-seeds what you just removed."
echo "  2. docker compose restart backend"
