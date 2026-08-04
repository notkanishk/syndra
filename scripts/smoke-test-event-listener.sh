#!/usr/bin/env bash
# scripts/smoke-test-event-listener.sh — POST a synthetic Zitadel event to
# /api/webhooks/zitadel with a valid ZITADEL-Signature header and assert 200.
#
# Deliberately uses an UNMAPPED Zitadel event type (`user.password.changed`)
# so the test exercises auth + shape detection + translator unknown-event
# passthrough without invoking any downstream processor. Mapped events
# (`user.human.added`, `user.grant.*`) would mutate onboarding/grant state
# even on staging — operators must not run smoke against production state.
#
# When ZITADEL_EVENT_SIGNING_KEY is set, the request is signed so the
# middleware accepts it in production mode. When unset, the endpoint is
# expected to be running in dev pass-through mode (warning logged, no
# verification).
#
# Usage:
#   scripts/smoke-test-event-listener.sh                       # hits local backend on :8080
#   scripts/smoke-test-event-listener.sh http://syndra:8080    # explicit host
#   BACKEND_URL=https://staging.example.com scripts/smoke-test-event-listener.sh
#
# Exit codes: 0 success; >0 failure (message on stderr).

set -euo pipefail

# ---- Auto-load .env from the repo root (if present) ----
# Same loader pattern as smoke-test-action-v2.sh / register.sh: explicit
# environment wins, silent when .env is absent.
_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_ENV_FILE="$(cd "${_SCRIPT_DIR}/.." && pwd)/.env"
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
unset _SCRIPT_DIR _ENV_FILE

HOST="${1:-${BACKEND_URL:-http://localhost:8080}}"

for bin in curl; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: $bin not installed" >&2; exit 1; }
done

# Synthetic Zitadel-shape event in the real ContextInfoEvent wire format
# (zitadel/zitadel:internal/repository/execution/queue.go) — flat top-level
# fields, snake_case for event_type/event_payload/created_at, userID is the
# editor (a smoke-test marker; only matches ZITADEL_M2M_USER_ID by accident).
# The event type is deliberately UNMAPPED so the translator's unknown-event
# branch fires (200 + log, no dispatch); this keeps the smoke test
# side-effect-free on production/staging.
PAYLOAD='{"aggregateID":"smoke-user-1","aggregateType":"user","resourceOwner":"org-1","instanceID":"inst","version":"v1","sequence":1,"event_type":"user.password.changed","created_at":"2026-05-07T00:00:00Z","userID":"smoke-operator","event_payload":{}}'

HEADERS=(-H 'Content-Type: application/json')
if [[ -n "${ZITADEL_EVENT_SIGNING_KEY:-}" ]]; then
  echo "Signing request with ZITADEL_EVENT_SIGNING_KEY..." >&2
  TS="$(date +%s)"
  if command -v python3 >/dev/null 2>&1; then
    SIG="$(python3 -c '
import os, hmac, hashlib, sys
key = os.environ["ZITADEL_EVENT_SIGNING_KEY"].encode()
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
  echo "note: ZITADEL_EVENT_SIGNING_KEY unset — assuming backend is in dev pass-through mode" >&2
fi

RESPONSE_FILE="$(mktemp)"
trap 'rm -f "$RESPONSE_FILE"' EXIT

HTTP_CODE="$(curl -sS -o "$RESPONSE_FILE" -w "%{http_code}" -X POST \
  "${HEADERS[@]}" \
  --data-raw "$PAYLOAD" \
  "${HOST}/api/webhooks/zitadel")"

if [[ "$HTTP_CODE" != "200" ]]; then
  echo "FAIL: expected HTTP 200, got ${HTTP_CODE}" >&2
  cat "$RESPONSE_FILE" >&2
  exit 3
fi

echo "OK: /api/webhooks/zitadel accepted synthetic unknown event (no dispatch)."
cat "$RESPONSE_FILE"
echo
