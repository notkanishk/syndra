#!/bin/bash
set -euo pipefail

# Required. The previous default pointed at a host that had been decommissioned,
# so an argument-less run smoke-tested nothing and said so cheerfully.
HOST="${1:?usage: smoke-test-lxc.sh <host> [ui-port] [api-port]}"
REMOTE_DIR="${SYNDRA_REMOTE_DIR:-/root/syndra}"

# The host ports come from the DEPLOYMENT's own .env — the same file compose
# reads BACKEND_HOST_PORT / UI_HOST_PORT from — and arguments override.
#
# Hardcoded at 3000/8080 this script checked the OTHER stack on a box running
# two and passed cheerfully. A smoke test that can pass against a deployment it
# was not pointed at is worse than no smoke test, because it is evidence.
#
# The fallback is deliberately narrow. "The .env has no override" means compose
# published 3000/8080 too, so the default is right. "The .env could not be read"
# means nothing is known about which ports this host publishes, and defaulting
# there is how the first version of this fix passed against the legacy stack —
# so that case fails instead.
if [ -z "${2:-}" ] || [ -z "${3:-}" ]; then
  ENV_DUMP=$(ssh "root@${HOST}" "cat ${REMOTE_DIR}/.env 2>/dev/null" || true)
  if [ -z "${ENV_DUMP}" ]; then
    echo "Cannot read ${REMOTE_DIR}/.env on ${HOST}, so the published ports are unknown." >&2
    echo "Pass them: smoke-test-lxc.sh ${HOST} <ui-port> <api-port>" >&2
    echo "(or set SYNDRA_REMOTE_DIR if the deployment lives elsewhere)" >&2
    exit 1
  fi
  env_port() { printf '%s\n' "${ENV_DUMP}" | grep -E "^$1=" | tail -1 | cut -d= -f2- | tr -d '\r'; }
fi

UI_PORT="${2:-$(env_port UI_HOST_PORT)}"
API_PORT="${3:-$(env_port BACKEND_HOST_PORT)}"
UI_PORT="${UI_PORT:-3000}"
API_PORT="${API_PORT:-8080}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:${UI_PORT}" >/dev/null

echo "Checking API availability..."
# /healthz is unauthenticated so this works on OIDC-mode deployments
# where the operator has no bearer token to hand the script.
curl -fsS "http://${HOST}:${API_PORT}/healthz" >/dev/null

echo "Checking Docker service status on remote host..."
ssh "root@${HOST}" "docker ps --format 'table {{.Names}}\t{{.Status}}'" 

echo "Smoke test passed for ${HOST} (ui :${UI_PORT}, api :${API_PORT})."
