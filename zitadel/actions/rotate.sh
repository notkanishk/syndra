#!/usr/bin/env bash
# zitadel/actions/rotate.sh — rotate the Actions v2 target signing key.
#
# Zitadel does not expire the Action target signing key (CreateTargetRequest
# has no expiration field; see proto/zitadel/action/v2/target.proto). The
# first key works indefinitely. Rotation is a MkAuth *policy* choice —
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
# MkAuth does NOT ship dual-key acceptance today — the backend trusts a
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

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/targets.json"
SIGNING_KEY_FILE="${SCRIPT_DIR}/.action-signing-key"
PREVIOUS_KEY_FILE="${SCRIPT_DIR}/.action-signing-key.previous"

# ---- Auto-load .env from the repo root (if present) ----
# Explicit environment wins: an already-set VAR is never overwritten by
# .env. Silent when .env is absent (CI, bare clone, container build).
# Parsing is deliberately narrow: KEY=VALUE lines with optional leading
# whitespace, optional `"…"`/`'…'` quotes stripped, `#` comments and
# blank lines ignored. `${VAR}` inside a value is kept literal — we don't
# re-implement shell expansion here.
_ENV_FILE="$(cd "${SCRIPT_DIR}/../.." && pwd)/.env"
if [[ -f "$_ENV_FILE" ]]; then
  while IFS= read -r _raw || [[ -n "$_raw" ]]; do
    [[ "$_raw" =~ ^[[:space:]]*($|#) ]] && continue
    [[ "$_raw" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    _k="${BASH_REMATCH[1]}"
    _v="${BASH_REMATCH[2]}"
    if [[ "$_v" =~ ^\"(.*)\"$ ]] || [[ "$_v" =~ ^\'(.*)\'$ ]]; then
      _v="${BASH_REMATCH[1]}"
    fi
    [[ -z "${!_k+x}" ]] && export "$_k=$_v"
  done < "$_ENV_FILE"
  unset _raw _k _v
fi
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
  REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
  case "$ZITADEL_MACHINE_KEY_PATH" in
    /*)    _abs_key="$ZITADEL_MACHINE_KEY_PATH" ;;
    '~/'*) _abs_key="$HOME/${ZITADEL_MACHINE_KEY_PATH#\~/}" ;;
    *)     _abs_key="$REPO_ROOT/$ZITADEL_MACHINE_KEY_PATH" ;;
  esac
  export ZITADEL_MACHINE_KEY_PATH="$_abs_key"
  unset _abs_key

  echo "Minting M2M token via backend/cmd/mkauth-token..." >&2
  BACKEND_DIR="${REPO_ROOT}/backend"
  if ! TOKEN="$(cd "$BACKEND_DIR" && go run ./cmd/mkauth-token)"; then
    echo "error: could not mint M2M token from ZITADEL_MACHINE_KEY_PATH — see mkauth-token stderr above, or provide ZITADEL_M2M_TOKEN directly" >&2
    exit 3
  fi
  if [[ -z "$TOKEN" ]]; then
    echo "error: mkauth-token returned an empty token" >&2
    exit 3
  fi
else
  echo "error: set ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH (in .env or export)" >&2
  exit 3
fi

API_BASE="https://${ZITADEL_DOMAIN}/v2/actions"

# zitadel_api METHOD PATH [JSON_BODY]
# Same helper shape as register.sh. On HTTP error prints Zitadel's response
# body verbatim so 401/403 permission failures surface the actual message
# instead of a bare "curl: (22)". Actions v2 target rotation requires
# IAM_OWNER on the service user; ORG_OWNER is not enough.
zitadel_api() {
  local method="$1" path="$2" body="${3:-}"
  local tmp status
  tmp="$(mktemp)"
  # shellcheck disable=SC2064
  trap "rm -f '$tmp'" RETURN
  local -a args=(-sS -o "$tmp" -w '%{http_code}'
    -X "$method" "${API_BASE}${path}"
    -H "Authorization: Bearer ${TOKEN}")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  status="$(curl "${args[@]}")"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    {
      printf 'error: %s %s -> HTTP %s\n' "$method" "$path" "$status"
      if [[ "$status" == "401" || "$status" == "403" ]]; then
        printf '       Actions v2 target management requires IAM_OWNER on the\n'
        printf '       service user. Assign it at Default Settings > Administrators\n'
        printf '       in the Zitadel console. ORG_OWNER is not sufficient.\n'
      fi
      printf 'response body:\n'
      cat "$tmp"
      printf '\n'
    } >&2
    return 1
  fi
  cat "$tmp"
}

# ---- Extract target name from manifest (strip _comment/_note annotations) ----
TARGET_NAME="$(jq -r '
  walk(
    if type == "object"
    then with_entries(select(.key | startswith("_") | not))
    else . end
  )
  | .target.name
' "$MANIFEST")"

if [[ -z "$TARGET_NAME" || "$TARGET_NAME" == "null" ]]; then
  echo "error: could not read .target.name from ${MANIFEST}" >&2
  exit 4
fi

# ---- Look up target ID by name ----
# TargetNameFilter fields per proto/zitadel/action/v2/query.proto: target_name + method.
echo "Searching for target name=${TARGET_NAME}..." >&2
SEARCH_BODY="$(jq -n --arg n "$TARGET_NAME" '{
  filters: [{ target_name_filter: { target_name: $n, method: "TEXT_FILTER_METHOD_EQUALS" } }],
  pagination: { limit: 1 }
}')"
LIST_RESP="$(zitadel_api POST /targets/search "$SEARCH_BODY")" || exit 5
TARGET_ID="$(echo "$LIST_RESP" | jq -r '.targets[0].id // .result[0].id // empty')"

if [[ -z "$TARGET_ID" || "$TARGET_ID" == "null" ]]; then
  echo "error: no target named '${TARGET_NAME}' found — nothing to rotate" >&2
  echo "       run 'make zitadel-actions-register' first" >&2
  exit 5
fi

# ---- Rotate: POST /v2/actions/targets/{id} with expirationSigningKey:0s ----
# Per proto UpdateTargetRequest.expiration_signing_key: current Zitadel only
# accepts "0s" (immediate hard swap). Longer graceful periods are a roadmap
# item; revisit this script if/when Zitadel supports them.
echo "Rotating signing key on target_id=${TARGET_ID}..." >&2
ROTATE_RESP="$(zitadel_api POST "/targets/${TARGET_ID}" '{"expirationSigningKey":"0s"}')" || exit 6

NEW_KEY="$(echo "$ROTATE_RESP" | jq -r '.signingKey // empty')"
if [[ -z "$NEW_KEY" ]]; then
  echo "error: UpdateTarget did not return a signingKey. Response was:" >&2
  echo "$ROTATE_RESP" >&2
  exit 6
fi

# ---- Back up previous key and write the new one (both mode 0600) ----
umask 077
if [[ -s "$SIGNING_KEY_FILE" ]]; then
  cp "$SIGNING_KEY_FILE" "$PREVIOUS_KEY_FILE"
  echo "Previous key backed up to ${PREVIOUS_KEY_FILE}" >&2
else
  echo "warning: no prior ${SIGNING_KEY_FILE} on disk — skipping backup" >&2
fi
printf '%s\n' "$NEW_KEY" > "$SIGNING_KEY_FILE"
echo "New signing key written to ${SIGNING_KEY_FILE} (mode 0600)." >&2

# ---- Capture rotation timestamp (RFC3339 UTC) for the Rotation Status panel ----
# GNU and BSD date both accept -u for UTC; the ISO-8601 format is portable.
ROTATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ROTATED_AT_FILE="${SCRIPT_DIR}/.action-signing-key.rotated_at"
printf '%s\n' "$ROTATED_AT" > "$ROTATED_AT_FILE"

# ---- Emit a ready-to-append env fragment ----
# Write both lines to a 0600 fragment file rather than asking the operator to
# copy-paste from the terminal. This removes every brittle path in the paste
# flow: shell substitution ambiguity, terminal line-wrap on long keys, stderr
# interleaving with other output, and the outside chance that a reader copies
# from the script source instead of its output. The operator runs a single
# `cat >> .env` (or equivalent) to apply the values atomically.
FRAGMENT_FILE="${SCRIPT_DIR}/.action-env.fragment"
{
  printf 'ZITADEL_ACTION_SIGNING_KEY=%s\n' "$NEW_KEY"
  printf 'ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=%s\n' "$ROTATED_AT"
} > "$FRAGMENT_FILE"

# ---- Operator guidance ----
cat >&2 <<EOF

Rotation complete. Apply the new values to your backend env:

    cat "${FRAGMENT_FILE}" >> .env
    # OR, for systemd EnvironmentFile deploys:
    sudo install -m 0600 "${FRAGMENT_FILE}" /etc/mkauth/action-env

Then restart the backend and verify:

    docker compose up -d backend       # or your deploy equivalent
    make zitadel-actions-verify

The /zitadel Rotation Status panel should flip to "ok" with age 0.

During the window between Zitadel accepting the new key and the backend
picking it up, inbound Action requests fail with 401 INVALID_SIGNATURE and
Zitadel falls back to stock claims. Because restCall.interruptOnError is
false, user token issuance is NOT blocked during this window — custom
claims simply disappear for the gap. Keep the restart under a minute.

The old key in .action-signing-key.previous is retained for audit/rollback.
The rotation timestamp is mirrored to .action-signing-key.rotated_at for
local audit; MkAuth itself reads ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT from
env at runtime, not this file.

Delete ${FRAGMENT_FILE} once the values are applied.
EOF

echo "Done. target_id=${TARGET_ID} rotated_at=${ROTATED_AT}" >&2
