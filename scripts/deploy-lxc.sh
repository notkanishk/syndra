#!/bin/bash
set -euo pipefail

REMOTE_HOST="${REMOTE_HOST:-root@198.51.100.14}"
REMOTE_DIR="${REMOTE_DIR:-/opt/mkauth}"
TMP_DIR="/tmp/mkauth-deploy"
ARCHIVE_NAME="mkauth-deploy.tar.gz"

echo "Preparing deployment archive from local workspace..."
COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar \
  --exclude=".git" \
  --exclude=".next" \
  --exclude="node_modules" \
  --exclude=".DS_Store" \
  --exclude="._*" \
  -czf "/tmp/${ARCHIVE_NAME}" \
  .

echo "Creating remote directories on ${REMOTE_HOST}..."
ssh "${REMOTE_HOST}" "mkdir -p '${REMOTE_DIR}' '${TMP_DIR}'"

echo "Uploading archive to ${REMOTE_HOST}..."
scp "/tmp/${ARCHIVE_NAME}" "${REMOTE_HOST}:${TMP_DIR}/${ARCHIVE_NAME}" >/dev/null

echo "Extracting release and restarting Docker Compose on ${REMOTE_HOST}..."
ssh "${REMOTE_HOST}" "
  set -euo pipefail
  rm -rf '${REMOTE_DIR}'
  mkdir -p '${REMOTE_DIR}'
  tar -xzf '${TMP_DIR}/${ARCHIVE_NAME}' -C '${REMOTE_DIR}'
  cd '${REMOTE_DIR}'
  docker compose build
  docker compose up -d
  docker image prune -f >/dev/null
"

echo "Deployment complete."
echo "UI:  http://198.51.100.14:3000"
echo "API: http://198.51.100.14:8080"
