#!/bin/bash
set -euo pipefail

HOST="${1:-198.51.100.14}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:3000" >/dev/null

echo "Checking API availability..."
# /healthz is unauthenticated so this works on OIDC-mode deployments
# where the operator has no bearer token to hand the script.
curl -fsS "http://${HOST}:8080/healthz" >/dev/null

echo "Checking Docker service status on remote host..."
ssh "root@${HOST}" "docker ps --format 'table {{.Names}}\t{{.Status}}'" 

echo "Smoke test passed for ${HOST}."
