#!/bin/bash
set -euo pipefail

HOST="${1:-198.51.100.14}"
API_KEY="${MKAUTH_API_KEY:-dev_auth_token_secret}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:3000" >/dev/null

echo "Checking API availability..."
curl -fsS \
  -H "Authorization: Bearer ${API_KEY}" \
  "http://${HOST}:8080/api/v1/bundles" >/dev/null

echo "Checking Docker service status on remote host..."
ssh "root@${HOST}" "docker ps --format 'table {{.Names}}\t{{.Status}}'" 

echo "Smoke test passed for ${HOST}."
