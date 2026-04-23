#!/usr/bin/env bash
# scripts/smoke-test-action-v2.sh — verify the Actions v2 data-plane endpoint
# responds with the expected Zitadel v2 envelope shape.
#
# Hits the running backend with a canonical v2 function trigger payload and
# asserts: (1) HTTP 200, (2) response contains an append_claims array, (3) the
# array is well-formed ([] or [{key,value}, ...]).
#
# When ZITADEL_ACTION_SIGNING_KEY is set in the environment, the request is
# signed with it so the middleware accepts it in production mode. Otherwise
# the endpoint is expected to be running in dev pass-through mode.
#
# Usage:
#   scripts/smoke-test-action-v2.sh                          # hits local backend on :8080
#   scripts/smoke-test-action-v2.sh http://mkauth:8080       # explicit host
#
# Exit codes: 0 success; >0 failure (message on stderr).

set -euo pipefail

HOST="${1:-http://localhost:8080}"

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

PAYLOAD='{"function":"function/preaccesstoken","user":{"id":"smoke-test-user"},"user_grants":[{"projectId":"smoke-test-project","roles":["viewer"]}]}'

HEADERS=(-H 'Content-Type: application/json')
if [[ -n "${ZITADEL_ACTION_SIGNING_KEY:-}" ]]; then
  echo "Signing request with ZITADEL_ACTION_SIGNING_KEY..." >&2
  TS="$(date +%s)"
  if command -v python3 >/dev/null 2>&1; then
    SIG="$(python3 -c '
import os, hmac, hashlib, sys
key = os.environ["ZITADEL_ACTION_SIGNING_KEY"].encode()
ts = sys.argv[1].encode()
body = sys.argv[2].encode()
mac = hmac.new(key, ts + b"." + body, hashlib.sha256).hexdigest()
print(mac)
' "$TS" "$PAYLOAD")"
  else
    echo "error: python3 required to compute HMAC signature" >&2
    exit 2
  fi
  HEADERS+=(-H "ZITADEL-Signature: t=${TS},v1=${SIG}")
else
  echo "note: ZITADEL_ACTION_SIGNING_KEY unset — assuming backend is in dev pass-through mode" >&2
fi

RESPONSE_FILE="$(mktemp)"
trap 'rm -f "$RESPONSE_FILE"' EXIT

HTTP_CODE="$(curl -sS -o "$RESPONSE_FILE" -w "%{http_code}" -X POST \
  "${HEADERS[@]}" \
  --data-raw "$PAYLOAD" \
  "${HOST}/api/action/inject")"

if [[ "$HTTP_CODE" != "200" ]]; then
  echo "FAIL: expected HTTP 200, got ${HTTP_CODE}" >&2
  cat "$RESPONSE_FILE" >&2
  exit 3
fi

if ! jq -e '.append_claims | type == "array"' <"$RESPONSE_FILE" >/dev/null; then
  echo "FAIL: response missing append_claims array" >&2
  cat "$RESPONSE_FILE" >&2
  exit 4
fi

# Each entry (if any) must have {key, value} — guards against regressions to
# v1-era `customClaims` shape.
if ! jq -e '.append_claims | all(has("key") and has("value"))' <"$RESPONSE_FILE" >/dev/null; then
  echo "FAIL: append_claims entries are not in v2 {key,value} form" >&2
  cat "$RESPONSE_FILE" >&2
  exit 5
fi

echo "OK: /api/action/inject returned a well-formed v2 envelope."
cat "$RESPONSE_FILE"
echo
