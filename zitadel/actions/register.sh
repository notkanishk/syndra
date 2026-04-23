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
#
# Required env:
#   ZITADEL_DOMAIN            e.g. your-instance.zitadel.cloud
#   MKAUTH_EXTERNAL_URL       Base URL Zitadel should POST to (e.g. https://mkauth.internal)
#
# Required one of (for M2M auth):
#   ZITADEL_M2M_TOKEN         Access token minted ahead of time
#   ZITADEL_MACHINE_KEY_PATH  Path to service-user key JSON (will mint a token via backend helper)
#
# Exit codes: 0 success; >0 failure (message on stderr).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/targets.json"
SIGNING_KEY_FILE="${SCRIPT_DIR}/.action-signing-key"

: "${ZITADEL_DOMAIN:?ZITADEL_DOMAIN is required}"
: "${MKAUTH_EXTERNAL_URL:?MKAUTH_EXTERNAL_URL is required (where Zitadel will POST)}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

# ---- Resolve M2M access token ----
if [[ -n "${ZITADEL_M2M_TOKEN:-}" ]]; then
  TOKEN="$ZITADEL_M2M_TOKEN"
elif [[ -n "${ZITADEL_MACHINE_KEY_PATH:-}" ]]; then
  echo "Minting M2M token via backend/cmd/test..." >&2
  # The backend ships a thin helper at cmd/test/main.go that prints a Bearer token.
  # When that helper does not exist, the operator must provide ZITADEL_M2M_TOKEN directly.
  TOKEN="$(cd "${SCRIPT_DIR}/../.." && go run ./backend/cmd/test -action=mint-m2m-token 2>/dev/null || true)"
  if [[ -z "$TOKEN" ]]; then
    echo "error: could not mint M2M token — provide ZITADEL_M2M_TOKEN directly" >&2
    exit 2
  fi
else
  echo "error: set ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH" >&2
  exit 2
fi

API_BASE="https://${ZITADEL_DOMAIN}/v2/actions"

# ---- Render targets.json ----
# Strip _comment/_note annotations (JSON-illegal per Zitadel's strict decoder)
# and substitute ${MKAUTH_EXTERNAL_URL} in any endpoint string.
RENDERED_MANIFEST="$(jq --arg url "$MKAUTH_EXTERNAL_URL" '
  walk(
    if type == "string" and test("\\$\\{MKAUTH_EXTERNAL_URL\\}")
    then sub("\\$\\{MKAUTH_EXTERNAL_URL\\}"; $url)
    else . end
  )
  | walk(
      if type == "object"
      then with_entries(select(.key | startswith("_") | not))
      else . end
    )
' "$MANIFEST")"

TARGET_NAME="$(echo "$RENDERED_MANIFEST" | jq -r '.target.name')"

# ---- Lookup existing target by name (idempotent upsert) ----
# TargetNameFilter in proto/zitadel/action/v2/query.proto has fields
# `target_name` and `method` (referencing zitadel.filter.v2.TextFilterMethod).
# Sending `name` here would silently match nothing and re-running the script
# would fall into the create path, producing duplicate targets instead of
# idempotent updates.
echo "Searching for target name=${TARGET_NAME}..." >&2
SEARCH_BODY="$(jq -n --arg n "$TARGET_NAME" '{
  filters: [{ target_name_filter: { target_name: $n, method: "TEXT_FILTER_METHOD_EQUALS" } }],
  pagination: { limit: 1 }
}')"
LIST_RESP="$(curl -fsS -X POST "${API_BASE}/targets/search" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d "$SEARCH_BODY" 2>/dev/null || true)"
EXISTING_ID="$(echo "$LIST_RESP" | jq -r '.targets[0].id // .result[0].id // empty' 2>/dev/null || true)"

# ---- --remove path: unbind executions, retain target + signing key ----
if [[ "${1:-}" == "--remove" ]]; then
  echo "Unbinding executions for target=${TARGET_NAME}..." >&2
  echo "$RENDERED_MANIFEST" | jq -c '.executions[].condition' | while read -r cond; do
    UNBIND_BODY="$(jq -n --argjson c "$cond" '{ condition: $c, targets: [] }')"
    curl -fsS -X PUT "${API_BASE}/executions" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$UNBIND_BODY" >/dev/null
  done
  echo "Executions unbound. Target retained (run 'DELETE ${API_BASE}/targets/${EXISTING_ID:-<id>}' or use the Zitadel console to fully remove)." >&2
  exit 0
fi

TARGET_BODY="$(echo "$RENDERED_MANIFEST" | jq '.target')"

if [[ -n "$EXISTING_ID" ]]; then
  echo "Updating target id=${EXISTING_ID}..." >&2
  curl -fsS -X POST "${API_BASE}/targets/${EXISTING_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$TARGET_BODY" >/dev/null
  TARGET_ID="$EXISTING_ID"
  if [[ ! -s "$SIGNING_KEY_FILE" ]]; then
    echo "warning: target exists but ${SIGNING_KEY_FILE} is missing." >&2
    echo "         The signing key is only returned at target-creation time." >&2
    echo "         To rotate: send POST ${API_BASE}/targets/${TARGET_ID} with {\"expirationSigningKey\":\"0s\"}," >&2
    echo "         capture the new signingKey from the response, and update ZITADEL_ACTION_SIGNING_KEY." >&2
  fi
else
  echo "Creating target name=${TARGET_NAME}..." >&2
  CREATE_RESP="$(curl -fsS -X POST "${API_BASE}/targets" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$TARGET_BODY")"
  TARGET_ID="$(echo "$CREATE_RESP" | jq -r '.id')"
  SIGNING_KEY="$(echo "$CREATE_RESP" | jq -r '.signingKey // empty')"
  if [[ -z "$TARGET_ID" || "$TARGET_ID" == "null" ]]; then
    echo "error: CreateTarget did not return an id. Response was:" >&2
    echo "$CREATE_RESP" >&2
    exit 3
  fi
  if [[ -z "$SIGNING_KEY" ]]; then
    echo "error: target created but no signingKey in response — aborting." >&2
    echo "$CREATE_RESP" >&2
    exit 4
  fi
  umask 077
  printf '%s\n' "$SIGNING_KEY" > "$SIGNING_KEY_FILE"
  echo "Signing key written to ${SIGNING_KEY_FILE} (mode 0600)." >&2
  echo "Inject it into the backend env as ZITADEL_ACTION_SIGNING_KEY before the next deploy." >&2
fi

echo "Binding executions to target id=${TARGET_ID}..." >&2
echo "$RENDERED_MANIFEST" | jq -c '.executions[]' | while read -r exec_entry; do
  BIND_BODY="$(echo "$exec_entry" | jq --arg tid "$TARGET_ID" '{
    condition: .condition,
    targets: [ $tid ]
  }')"
  curl -fsS -X PUT "${API_BASE}/executions" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "$BIND_BODY" >/dev/null
done

echo "Done. target_id=${TARGET_ID}." >&2
