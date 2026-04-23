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
: "${MKAUTH_EXTERNAL_URL:?MKAUTH_EXTERNAL_URL is required (where Zitadel will POST; set in .env or export)}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

# ---- Resolve M2M access token ----
# Order of preference: an explicit ZITADEL_M2M_TOKEN (useful for CI or ops
# hosts without a Go toolchain), else mint one from the service-account key
# via `backend/cmd/mkauth-token`. The mint helper must run inside the
# `backend` module root so `go run` can resolve imports — which means we
# first need to pin ZITADEL_MACHINE_KEY_PATH to an absolute path, because
# `.env.example` documents it as "relative paths resolve against the
# docker-compose.yml directory" (i.e. the repo root) and `cd backend`
# would otherwise reinterpret `./zitadel-machine-key.json` as
# `backend/zitadel-machine-key.json`.
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
    exit 2
  fi
  if [[ -z "$TOKEN" ]]; then
    echo "error: mkauth-token returned an empty token" >&2
    exit 2
  fi
else
  echo "error: set ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH (in .env or export)" >&2
  exit 2
fi

API_BASE="https://${ZITADEL_DOMAIN}/v2/actions"

# zitadel_api METHOD PATH [JSON_BODY]
# Authenticated call against Zitadel's v2 Actions API. On 2xx, prints the
# response body on stdout. On 4xx/5xx, prints method + path + status +
# Zitadel's own JSON error on stderr and returns non-zero — so the operator
# sees what Zitadel actually complained about instead of a bare
# "curl: (22)". 401/403 get an IAM_OWNER hint inline because Actions v2
# target management requires instance-level permission that ORG_OWNER does
# not cover; see DEPLOY.md "Prerequisites".
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
LIST_RESP="$(zitadel_api POST /targets/search "$SEARCH_BODY")" || exit 5
EXISTING_ID="$(echo "$LIST_RESP" | jq -r '.targets[0].id // .result[0].id // empty' 2>/dev/null || true)"

# ---- --remove path: unbind executions, retain target + signing key ----
if [[ "${1:-}" == "--remove" ]]; then
  echo "Unbinding executions for target=${TARGET_NAME}..." >&2
  echo "$RENDERED_MANIFEST" | jq -c '.executions[].condition' | while read -r cond; do
    UNBIND_BODY="$(jq -n --argjson c "$cond" '{ condition: $c, targets: [] }')"
    zitadel_api PUT /executions "$UNBIND_BODY" >/dev/null || exit 6
  done
  echo "Executions unbound. Target retained (run 'DELETE ${API_BASE}/targets/${EXISTING_ID:-<id>}' or use the Zitadel console to fully remove)." >&2
  exit 0
fi

TARGET_BODY="$(echo "$RENDERED_MANIFEST" | jq '.target')"

if [[ -n "$EXISTING_ID" ]]; then
  echo "Updating target id=${EXISTING_ID}..." >&2
  zitadel_api POST "/targets/${EXISTING_ID}" "$TARGET_BODY" >/dev/null || exit 7
  TARGET_ID="$EXISTING_ID"
  if [[ ! -s "$SIGNING_KEY_FILE" ]]; then
    echo "warning: target exists but ${SIGNING_KEY_FILE} is missing." >&2
    echo "         The signing key is only returned at target-creation time." >&2
    echo "         To rotate: send POST ${API_BASE}/targets/${TARGET_ID} with {\"expirationSigningKey\":\"0s\"}," >&2
    echo "         capture the new signingKey from the response, and update ZITADEL_ACTION_SIGNING_KEY." >&2
  fi
else
  echo "Creating target name=${TARGET_NAME}..." >&2
  CREATE_RESP="$(zitadel_api POST /targets "$TARGET_BODY")" || exit 8
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
  zitadel_api PUT /executions "$BIND_BODY" >/dev/null || exit 9
done

echo "Done. target_id=${TARGET_ID}." >&2
