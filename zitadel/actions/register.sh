#!/usr/bin/env bash
# zitadel/actions/register.sh — apply targets.json against a live Zitadel instance.
#
# Speaks the stable Zitadel Actions v2 REST API (https://.../v2/actions/*)
# verified against: Target/Execution/Query proto at
# zitadel/zitadel:proto/zitadel/action/v2 (NOT v2beta — the predecessor
# still exists in the repo but the current wire path is v2 stable) and the
# testing-function walkthrough at
# zitadel.com/docs/guides/integrate/actions/testing-function.
#
# Verbs and bodies:
#   Create target:       POST   /v2/actions/targets           body: {name, endpoint, timeout, restCall:{interruptOnError}}
#   Update target:       POST   /v2/actions/targets/{id}      body: same (partial)
#   Search targets:      POST   /v2/actions/targets/search    body: {filters:[{target_name_filter:{name}}], pagination}
#   Bind / unbind exec:  PUT    /v2/actions/executions        body: {condition:{function:{name}}, targets:[ids|[]]}
#
# Target type for claim injection is restCall — webhook targets only use the
# status code; we need Zitadel to consume the response body as the envelope.
#
# Idempotent: re-running upserts the target by name. First successful create
# captures the one-time signing key to zitadel/actions/.action-signing-key
# (mode 0600). Zitadel does not re-issue the key on subsequent reads.
#
# Usage:
#   register.sh               # create/update target, bind executions
#   register.sh --remove      # unbind executions (target + key retained)
#   register.sh --purge       # unbind executions AND delete every manifest
#                             # target via DELETE /v2/actions/targets/{id};
#                             # also deletes local .action-signing-key.<name>
#                             # files so a subsequent register.sh starts from
#                             # a clean slate. Destructive: re-registering
#                             # mints fresh signing keys, requiring a backend
#                             # env-swap and restart.
#
# Required env:
#   ZITADEL_DOMAIN            e.g. your-instance.zitadel.cloud
#   SYNDRA_EXTERNAL_URL       Base URL Zitadel should POST to (e.g. https://syndra.internal)
#
# Required one of (for M2M auth):
#   ZITADEL_M2M_TOKEN         Access token minted ahead of time
#   ZITADEL_MACHINE_KEY_PATH  Path to service-user key JSON (will mint a token via backend helper)
#
# Exit codes: 0 success; >0 failure (message on stderr).

set -euo pipefail

if (( BASH_VERSINFO[0] < 4 )); then
  echo "error: register.sh requires bash 4+ (associative arrays)" >&2
  echo "  macOS default is bash 3; install via 'brew install bash' and rerun." >&2
  exit 1
fi

# Three modes: apply (default), remove (unbind only), purge (unbind + delete
# targets + delete local signing-key files). remove and purge share the
# unbind path; purge then runs an extra target-delete + key-file-cleanup
# pass after the loop.
MODE="apply"
case "${1:-}" in
  "")        MODE="apply"  ;;
  --remove)  MODE="remove" ;;
  --purge)   MODE="purge"  ;;
  *)         echo "error: unknown argument '${1}' (expected --remove or --purge)" >&2; exit 1 ;;
esac
DESTRUCTIVE_MODE=0
[[ "$MODE" == "remove" || "$MODE" == "purge" ]] && DESTRUCTIVE_MODE=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST="${SCRIPT_DIR}/targets.json"

# ---- Auto-load .env from the repo root (if present) ----
# Explicit environment wins: an already-set VAR is never overwritten by
# .env. Silent when .env is absent (CI, bare clone, container build). The
# loader logic lives in scripts/lib/load-env.sh (shared with rotate.sh).
_ENV_FILE="${REPO_ROOT}/.env"
# shellcheck source=../../scripts/lib/load-env.sh
source "${REPO_ROOT}/scripts/lib/load-env.sh"
unset _ENV_FILE

: "${ZITADEL_DOMAIN:?ZITADEL_DOMAIN is required (set in .env or export)}"
: "${SYNDRA_EXTERNAL_URL:?SYNDRA_EXTERNAL_URL is required (where Zitadel will POST; set in .env or export)}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

# ---- Resolve M2M access token ----
# Order of preference: an explicit ZITADEL_M2M_TOKEN (useful for CI or ops
# hosts without a Go toolchain), else mint one from the service-account key
# via `backend/cmd/syndra-token`. The mint helper must run inside the
# `backend` module root so `go run` can resolve imports — which means we
# first need to pin ZITADEL_MACHINE_KEY_PATH to an absolute path, because
# `.env.example` documents it as "relative paths resolve against the
# docker-compose.yml directory" (i.e. the repo root) and `cd backend`
# would otherwise reinterpret `./zitadel-machine-key.json` as
# `backend/zitadel-machine-key.json`.
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
    exit 2
  fi
  if [[ -z "$TOKEN" ]]; then
    echo "error: syndra-token returned an empty token" >&2
    exit 2
  fi
else
  echo "error: set ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH (in .env or export)" >&2
  exit 2
fi

API_BASE="https://${ZITADEL_DOMAIN}/v2/actions"

# zitadel_api METHOD PATH [JSON_BODY] — shared helper, expects API_BASE + TOKEN
# in scope (both established above) and honours optional ZITADEL_API_TOLERATE_404.
# shellcheck source=../../scripts/lib/zitadel-api.sh
source "${REPO_ROOT}/scripts/lib/zitadel-api.sh"

# ---- Render targets.json ----
# Strip _comment/_note annotations (JSON-illegal per Zitadel's strict decoder)
# and substitute ${SYNDRA_EXTERNAL_URL} in any endpoint string.
RENDERED_MANIFEST="$(jq --arg url "$SYNDRA_EXTERNAL_URL" '
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

# Map: target name -> registered/looked-up target ID. Populated as we walk
# .targets[]; consumed when binding executions.
declare -A TARGET_IDS

# Process each target in the manifest.
TARGET_COUNT="$(echo "$RENDERED_MANIFEST" | jq '.targets | length')"
for ((i = 0; i < TARGET_COUNT; i++)); do
  T="$(echo "$RENDERED_MANIFEST" | jq -c ".targets[$i]")"
  TARGET_NAME="$(echo "$T" | jq -r '.name')"
  SIGNING_KEY_ENV="$(echo "$T" | jq -r '._signing_key_env // empty')"
  TARGET_BODY="$(echo "$T" | jq 'del(._signing_key_env)')"
  SIGNING_KEY_FILE="${SCRIPT_DIR}/.action-signing-key.${TARGET_NAME}"

  echo "Searching for target name=${TARGET_NAME}..." >&2
  SEARCH_BODY="$(jq -n --arg n "$TARGET_NAME" '{
    filters: [{ target_name_filter: { target_name: $n, method: "TEXT_FILTER_METHOD_EQUALS" } }],
    pagination: { limit: 1 }
  }')"
  LIST_RESP="$(zitadel_api POST /targets/search "$SEARCH_BODY")" || exit 5
  EXISTING_ID="$(echo "$LIST_RESP" | jq -r '.targets[0].id // .result[0].id // empty' 2>/dev/null || true)"

  if (( DESTRUCTIVE_MODE )); then
    # Remove/purge path: just record IDs so the unbind loop below (and the
    # purge target-delete pass) can reach them.
    [[ -n "$EXISTING_ID" ]] && TARGET_IDS[$TARGET_NAME]="$EXISTING_ID"
    continue
  fi

  if [[ -n "$EXISTING_ID" ]]; then
    echo "Updating target id=${EXISTING_ID} name=${TARGET_NAME}..." >&2
    zitadel_api POST "/targets/${EXISTING_ID}" "$TARGET_BODY" >/dev/null || exit 7
    TARGET_IDS[$TARGET_NAME]="$EXISTING_ID"
    if [[ ! -s "$SIGNING_KEY_FILE" ]]; then
      echo "warning: target ${TARGET_NAME} exists but ${SIGNING_KEY_FILE} is missing." >&2
      echo "         The signing key is only returned at target-creation time." >&2
      echo "         Rotate via: make zitadel-actions-rotate-key TARGET=${TARGET_NAME}" >&2
    fi
  else
    echo "Creating target name=${TARGET_NAME}..." >&2
    CREATE_RESP="$(zitadel_api POST /targets "$TARGET_BODY")" || exit 8
    TARGET_ID="$(echo "$CREATE_RESP" | jq -r '.id')"
    SIGNING_KEY="$(echo "$CREATE_RESP" | jq -r '.signingKey // empty')"
    if [[ -z "$TARGET_ID" || "$TARGET_ID" == "null" ]]; then
      echo "error: CreateTarget did not return an id for ${TARGET_NAME}. Response was:" >&2
      echo "$CREATE_RESP" >&2
      exit 3
    fi
    if [[ -z "$SIGNING_KEY" ]]; then
      echo "error: target ${TARGET_NAME} created but no signingKey in response — aborting." >&2
      echo "$CREATE_RESP" >&2
      exit 4
    fi
    umask 077
    printf '%s\n' "$SIGNING_KEY" > "$SIGNING_KEY_FILE"
    echo "Signing key for ${TARGET_NAME} written to ${SIGNING_KEY_FILE} (mode 0600)." >&2
    TARGET_IDS[$TARGET_NAME]="$TARGET_ID"

    # Append to env-fragment file so operators can apply both keys with one
    # `cat .action-env.fragment >> .env`.
    FRAGMENT_FILE="${SCRIPT_DIR}/.action-env.fragment"
    if [[ -n "$SIGNING_KEY_ENV" ]]; then
      umask 077
      printf '%s=%s\n' "$SIGNING_KEY_ENV" "$SIGNING_KEY" >> "$FRAGMENT_FILE"
      printf '%s=%s\n' "${SIGNING_KEY_ENV}_ROTATED_AT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$FRAGMENT_FILE"
      echo "  → appended ${SIGNING_KEY_ENV} to ${FRAGMENT_FILE}" >&2
    fi
  fi
done

# --- Bind executions to the right target IDs ---
echo "Binding executions..." >&2
EXEC_COUNT="$(echo "$RENDERED_MANIFEST" | jq '.executions | length')"
for ((i = 0; i < EXEC_COUNT; i++)); do
  EXEC="$(echo "$RENDERED_MANIFEST" | jq -c ".executions[$i]")"
  TARGET_NAME="$(echo "$EXEC" | jq -r '.target')"
  COND="$(echo "$EXEC" | jq -c '.condition')"

  if (( DESTRUCTIVE_MODE )); then
    # Unbind by condition alone — Zitadel's PUT /executions with targets:[]
    # doesn't need the target ID. Tolerate a partially-deleted (or
    # never-created) target so cleanup still completes. Tolerate 404 too:
    # Zitadel returns HTTP 404 (COMMAND-74aaqj8fv9) when no execution row
    # matches the condition, which is the desired post-state for removal.
    BIND_BODY="$(jq -n --argjson c "$COND" '{ condition: $c, targets: [] }')"
    ZITADEL_API_TOLERATE_404=1 zitadel_api PUT /executions "$BIND_BODY" >/dev/null || exit 10
  else
    TARGET_ID="${TARGET_IDS[$TARGET_NAME]:-}"
    if [[ -z "$TARGET_ID" ]]; then
      echo "error: execution references unknown target=${TARGET_NAME}" >&2
      exit 9
    fi
    BIND_BODY="$(jq -n --argjson c "$COND" --arg tid "$TARGET_ID" '{ condition: $c, targets: [$tid] }')"
    zitadel_api PUT /executions "$BIND_BODY" >/dev/null || exit 10
  fi
done

# --- Purge-only: delete every manifest target and its local signing-key file ---
# Runs after the unbind loop so executions are gone before targets disappear
# (Zitadel will refuse to delete a target still referenced by an execution).
# Tolerates 404 on DELETE so re-running --purge against an already-clean
# instance is idempotent. Local key-file deletion is best-effort: missing
# files are fine, the .previous companion is also removed so the next
# register.sh starts truly fresh.
if [[ "$MODE" == "purge" ]]; then
  echo "Deleting targets..." >&2
  for ((i = 0; i < TARGET_COUNT; i++)); do
    T="$(echo "$RENDERED_MANIFEST" | jq -c ".targets[$i]")"
    TARGET_NAME="$(echo "$T" | jq -r '.name')"
    TARGET_ID="${TARGET_IDS[$TARGET_NAME]:-}"
    SIGNING_KEY_FILE="${SCRIPT_DIR}/.action-signing-key.${TARGET_NAME}"
    if [[ -n "$TARGET_ID" ]]; then
      echo "  → DELETE /targets/${TARGET_ID} (${TARGET_NAME})" >&2
      ZITADEL_API_TOLERATE_404=1 zitadel_api DELETE "/targets/${TARGET_ID}" >/dev/null || exit 11
    else
      echo "  → ${TARGET_NAME}: no target id (already absent in Zitadel)" >&2
    fi
    rm -f -- "$SIGNING_KEY_FILE" "${SIGNING_KEY_FILE}.previous" "${SIGNING_KEY_FILE}.rotated_at"
  done
  rm -f -- "${SCRIPT_DIR}/.action-env.fragment"
fi

echo "Done." >&2
echo "Targets: $(echo "${!TARGET_IDS[@]}" | tr ' ' ',')" >&2
if [[ "$MODE" == "apply" ]] && [[ -f "${SCRIPT_DIR}/.action-env.fragment" ]]; then
  echo "" >&2
  echo "Apply captured signing keys to .env with:" >&2
  echo "  cat zitadel/actions/.action-env.fragment >> .env" >&2
  echo "Then restart the backend so it picks up the new env vars." >&2
fi
if [[ "$MODE" == "purge" ]]; then
  echo "" >&2
  echo "Targets deleted. Local signing-key files removed:" >&2
  echo "  ${SCRIPT_DIR}/.action-signing-key.<name>{,.previous,.rotated_at}" >&2
  echo "Remember to clear ZITADEL_ACTION_SIGNING_KEY / ZITADEL_EVENT_SIGNING_KEY" >&2
  echo "from .env (and restart the backend) before re-registering — re-register.sh" >&2
  echo "will mint fresh keys that won't match the old env values." >&2
fi
