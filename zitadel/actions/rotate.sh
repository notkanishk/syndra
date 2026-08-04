#!/usr/bin/env bash
# zitadel/actions/rotate.sh — rotate the Actions v2 target signing key.
#
# Zitadel does not expire the Action target signing key (CreateTargetRequest
# has no expiration field; see proto/zitadel/action/v2/target.proto). The
# first key works indefinitely. Rotation is a Syndra *policy* choice —
# incident response, compliance hygiene, credential suspicion, operator
# handoff — not a Zitadel requirement.
#
# Wire flow:
#   1. POST /v2/actions/targets/search         — look up target id by name
#   2. POST /v2/actions/targets/{id}           — body: {"expirationSigningKey":"0s"}
#                                                response carries the new `signingKey`
#
# File flow:
#   - existing .action-signing-key  -> .action-signing-key.previous  (backup, mode 0600)
#   - new key                       -> .action-signing-key           (mode 0600)
#
# Syndra does NOT ship dual-key acceptance today — the backend trusts a
# single ZITADEL_ACTION_SIGNING_KEY env var. After this script succeeds the
# operator must swap the env var and restart the backend. During the swap
# window, inbound Action requests fail 401 INVALID_SIGNATURE and Zitadel
# issues tokens with stock claims only (because restCall.interruptOnError is
# false). Users are never blocked; custom claims disappear for the gap.
#
# Usage:
#   rotate.sh
#
# Required env:
#   ZITADEL_DOMAIN            e.g. your-instance.zitadel.cloud
#
# Required one of (for M2M auth):
#   ZITADEL_M2M_TOKEN         Access token minted ahead of time
#   ZITADEL_MACHINE_KEY_PATH  Path to service-user key JSON (mints a token via the backend helper)
#
# Exit codes: 0 success; >0 failure (message on stderr).

set -euo pipefail

if (( BASH_VERSINFO[0] < 4 )); then
  echo "error: rotate.sh requires bash 4+ (associative arrays)" >&2
  echo "  macOS default is bash 3; install via 'brew install bash' and rerun." >&2
  exit 1
fi

# Optional --target NAME filter. Default: rotate every target in the manifest.
TARGET_FILTER=""
case "${1:-}" in
  --target) TARGET_FILTER="${2:?--target requires a name argument}"; shift 2 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST="${SCRIPT_DIR}/targets.json"
FRAGMENT_FILE="${SCRIPT_DIR}/.action-env.fragment"

# ---- Auto-load .env from the repo root (if present) ----
# Explicit environment wins: an already-set VAR is never overwritten by
# .env. Silent when .env is absent (CI, bare clone, container build). The
# loader logic lives in scripts/lib/load-env.sh (shared with register.sh).
_ENV_FILE="${REPO_ROOT}/.env"
# shellcheck source=../../scripts/lib/load-env.sh
source "${REPO_ROOT}/scripts/lib/load-env.sh"
unset _ENV_FILE

: "${ZITADEL_DOMAIN:?ZITADEL_DOMAIN is required (set in .env or export)}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

if [[ ! -s "$MANIFEST" ]]; then
  echo "error: ${MANIFEST} missing — run register.sh first to create the target" >&2
  exit 2
fi

# ---- Resolve M2M access token (same pattern as register.sh) ----
# Relative ZITADEL_MACHINE_KEY_PATH values are resolved against the repo root
# per .env.example's documented contract, BEFORE we cd into backend/ for the
# `go run` module-root resolution. Without this, `./zitadel-machine-key.json`
# would be interpreted as `backend/./zitadel-machine-key.json`.
if [[ -n "${ZITADEL_M2M_TOKEN:-}" ]]; then
  TOKEN="$ZITADEL_M2M_TOKEN"
elif [[ -n "${ZITADEL_MACHINE_KEY_PATH:-}" ]]; then
  case "$ZITADEL_MACHINE_KEY_PATH" in
    /*)    _abs_key="$ZITADEL_MACHINE_KEY_PATH" ;;
    '~/'*) _abs_key="$HOME/${ZITADEL_MACHINE_KEY_PATH#\~/}" ;;
    *)     _abs_key="$REPO_ROOT/$ZITADEL_MACHINE_KEY_PATH" ;;
  esac
  export ZITADEL_MACHINE_KEY_PATH="$_abs_key"
  unset _abs_key

  echo "Minting M2M token via backend/cmd/syndra-token..." >&2
  BACKEND_DIR="${REPO_ROOT}/backend"
  if ! TOKEN="$(cd "$BACKEND_DIR" && go run ./cmd/syndra-token)"; then
    echo "error: could not mint M2M token from ZITADEL_MACHINE_KEY_PATH — see syndra-token stderr above, or provide ZITADEL_M2M_TOKEN directly" >&2
    exit 3
  fi
  if [[ -z "$TOKEN" ]]; then
    echo "error: syndra-token returned an empty token" >&2
    exit 3
  fi
else
  echo "error: set ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH (in .env or export)" >&2
  exit 3
fi

API_BASE="https://${ZITADEL_DOMAIN}/v2/actions"

# zitadel_api METHOD PATH [JSON_BODY] — shared helper, expects API_BASE + TOKEN
# in scope (both established above) and honours optional ZITADEL_API_TOLERATE_404.
# shellcheck source=../../scripts/lib/zitadel-api.sh
source "${REPO_ROOT}/scripts/lib/zitadel-api.sh"

# rotate_target NAME ENV_VAR_NAME
# Wire flow per target:
#   POST /v2/actions/targets/search    body: {filters:[{target_name_filter:{...}}], pagination}
#   POST /v2/actions/targets/{id}      body: {"expirationSigningKey":"0s"}
# Files written (all mode 0600, per-target so multi-target rotations don't
# collide):
#   .action-signing-key.<name>             new signing key
#   .action-signing-key.<name>.previous    backup of prior key (when present)
#   .action-signing-key.<name>.rotated_at  RFC3339 UTC timestamp
# Appends two lines to FRAGMENT_FILE:
#   <ENV_VAR>=<key>
#   <ENV_VAR>_ROTATED_AT=<timestamp>
# When ENV_VAR_NAME is empty the fragment lines are skipped (manifest target
# has no _signing_key_env hint — backend isn't expected to consume the key).
rotate_target() {
  local name="$1" env_var="$2"
  local key_file="${SCRIPT_DIR}/.action-signing-key.${name}"
  local prev_file="${SCRIPT_DIR}/.action-signing-key.${name}.previous"
  local rotated_at_file="${SCRIPT_DIR}/.action-signing-key.${name}.rotated_at"

  echo "Searching for target name=${name}..." >&2
  local search_body list_resp target_id
  search_body="$(jq -n --arg n "$name" '{
    filters: [{ target_name_filter: { target_name: $n, method: "TEXT_FILTER_METHOD_EQUALS" } }],
    pagination: { limit: 1 }
  }')"
  list_resp="$(zitadel_api POST /targets/search "$search_body")" || exit 5
  target_id="$(echo "$list_resp" | jq -r '.targets[0].id // .result[0].id // empty')"

  if [[ -z "$target_id" || "$target_id" == "null" ]]; then
    echo "error: no target named '${name}' found — nothing to rotate" >&2
    echo "       run 'make zitadel-actions-register' first" >&2
    exit 5
  fi

  # Per proto UpdateTargetRequest.expiration_signing_key: current Zitadel only
  # accepts "0s" (immediate hard swap). Longer graceful periods are a roadmap
  # item; revisit if/when Zitadel supports them.
  echo "Rotating signing key on target_id=${target_id} name=${name}..." >&2
  local rotate_resp new_key
  rotate_resp="$(zitadel_api POST "/targets/${target_id}" '{"expirationSigningKey":"0s"}')" || exit 6
  new_key="$(echo "$rotate_resp" | jq -r '.signingKey // empty')"
  if [[ -z "$new_key" ]]; then
    echo "error: UpdateTarget did not return a signingKey for ${name}. Response was:" >&2
    echo "$rotate_resp" >&2
    exit 6
  fi

  umask 077
  if [[ -s "$key_file" ]]; then
    cp "$key_file" "$prev_file"
    echo "Previous key for ${name} backed up to ${prev_file}" >&2
  else
    echo "warning: no prior ${key_file} on disk — skipping backup" >&2
  fi
  printf '%s\n' "$new_key" > "$key_file"
  echo "New signing key for ${name} written to ${key_file} (mode 0600)." >&2

  local rotated_at
  rotated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\n' "$rotated_at" > "$rotated_at_file"

  if [[ -n "$env_var" ]]; then
    {
      printf '%s=%s\n' "$env_var" "$new_key"
      printf '%s=%s\n' "${env_var}_ROTATED_AT" "$rotated_at"
    } >> "$FRAGMENT_FILE"
    echo "  → appended ${env_var} to ${FRAGMENT_FILE}" >&2
  fi

  echo "Rotated ${name}: target_id=${target_id} rotated_at=${rotated_at}" >&2
}

# ---- Render manifest (strip _comment/_note annotations, preserve _signing_key_env) ----
RENDERED_MANIFEST="$(jq --arg url "${SYNDRA_EXTERNAL_URL:-}" '
  walk(
    if type == "string" and test("\\$\\{SYNDRA_EXTERNAL_URL\\}")
    then sub("\\$\\{SYNDRA_EXTERNAL_URL\\}"; $url)
    else . end
  )
  | walk(
      if type == "object"
      then with_entries(select(
        (.key | startswith("_") | not) or .key == "_signing_key_env"
      ))
      else . end
    )
' "$MANIFEST")"

# Reset the fragment file at the start of each rotation run so it only carries
# the freshly-rotated keys. Operator-guidance below points at this single file.
umask 077
: > "$FRAGMENT_FILE"

# ---- Dispatch: multi-target manifest (.targets[]) or legacy single-target (.target) ----
if echo "$RENDERED_MANIFEST" | jq -e '.targets' >/dev/null; then
  COUNT="$(echo "$RENDERED_MANIFEST" | jq '.targets | length')"
  ANY_MATCHED=0
  for ((i = 0; i < COUNT; i++)); do
    T="$(echo "$RENDERED_MANIFEST" | jq -c ".targets[$i]")"
    NAME="$(echo "$T" | jq -r '.name')"
    if [[ -n "$TARGET_FILTER" && "$NAME" != "$TARGET_FILTER" ]]; then continue; fi
    ENV_VAR="$(echo "$T" | jq -r '._signing_key_env // empty')"
    rotate_target "$NAME" "$ENV_VAR"
    ANY_MATCHED=1
  done
  if [[ -n "$TARGET_FILTER" && "$ANY_MATCHED" -eq 0 ]]; then
    echo "error: --target ${TARGET_FILTER} did not match any target in ${MANIFEST}" >&2
    exit 7
  fi
else
  NAME="$(echo "$RENDERED_MANIFEST" | jq -r '.target.name')"
  if [[ -z "$NAME" || "$NAME" == "null" ]]; then
    echo "error: could not read .target.name from ${MANIFEST}" >&2
    exit 4
  fi
  rotate_target "$NAME" "ZITADEL_ACTION_SIGNING_KEY"
fi

# ---- Operator guidance (single-shot, regardless of target count) ----
cat >&2 <<EOF

Rotation complete. Apply the new values to your backend env:

    cat "${FRAGMENT_FILE}" >> .env
    # OR, for systemd EnvironmentFile deploys:
    sudo install -m 0600 "${FRAGMENT_FILE}" /etc/syndra/action-env

Then restart the backend and verify:

    docker compose up -d backend       # or your deploy equivalent
    make zitadel-actions-verify

The /zitadel Rotation Status panel should flip to "ok" with age 0.

During the window between Zitadel accepting the new key and the backend
picking it up, inbound Action requests fail with 401 INVALID_SIGNATURE and
Zitadel falls back to stock claims. Because restCall.interruptOnError is
false, user token issuance is NOT blocked during this window — custom
claims simply disappear for the gap. Keep the restart under a minute.

Per-target backups in .action-signing-key.<name>.previous are retained for
audit/rollback. Rotation timestamps are mirrored to
.action-signing-key.<name>.rotated_at for local audit; Syndra itself reads
the *_ROTATED_AT env vars at runtime, not these files.

Delete ${FRAGMENT_FILE} once the values are applied.
EOF

echo "Done." >&2
