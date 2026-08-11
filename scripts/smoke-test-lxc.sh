#!/bin/bash
set -euo pipefail

# Required. The previous default pointed at a host that had been decommissioned,
# so an argument-less run smoke-tested nothing and said so cheerfully.
HOST="${1:?usage: smoke-test-lxc.sh <host> [ui-port] [api-port]}"
# The host ports are arguments because they are configurable: a box running a
# second stack sets BACKEND_HOST_PORT / UI_HOST_PORT in its .env, and a smoke
# test hardcoding 3000/8080 checks the OTHER deployment and passes.
UI_PORT="${2:-3000}"
API_PORT="${3:-8080}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:${UI_PORT}" >/dev/null

echo "Checking API availability..."
# /healthz is unauthenticated so this works on OIDC-mode deployments
# where the operator has no bearer token to hand the script.
curl -fsS "http://${HOST}:${API_PORT}/healthz" >/dev/null

echo "Checking Docker service status on remote host..."
ssh "root@${HOST}" "docker ps --format 'table {{.Names}}\t{{.Status}}'" 

echo "Smoke test passed for ${HOST} (ui :${UI_PORT}, api :${API_PORT})."
