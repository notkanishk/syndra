#!/bin/bash
set -euo pipefail

# Put this commit's tracked source on a deployment host — and prove nothing the
# commit deleted survived there.
#
# It exists because of a real failure. Syncing with `cp -a` could add files and
# change files and never remove one, so `ui/src/app/system/hardware-sync` stayed
# on the box after the repo deleted it, the rebuilt image kept serving the
# retired route, and `/system/hardware-sync` answered 200 against a commit that
# had removed it. Worse than the stale route: a green deploy was reported as
# verifying a fix it had never carried.
#
# So the sync MIRRORS — the tracked directories are removed and re-extracted,
# not copied over — and then it checks. The check is the part that matters: an
# add-only sync looks identical to a correct one from every angle except the one
# nobody looks at, which is whether anything is present on the target that the
# repo does not have.
#
# What it deliberately does not touch: `.env`, `secrets/`, and anything else the
# host owns. Deployment configuration belongs to the deployment. Host ports live
# in `.env` as BACKEND_HOST_PORT / UI_HOST_PORT for exactly this reason — they
# used to be edited into docker-compose.yml, which is tracked, and the next
# sync overwrote them.
#
# Usage: scripts/deploy-source.sh root@198.51.100.16 [/root/syndra]

TARGET="${1:?usage: deploy-source.sh <user@host> [remote-dir]}"
REMOTE_DIR="${2:-/root/syndra}"
STAGING="/tmp/syndra-deploy-$$"

# The tracked top-level entries that make up a deployment. Named rather than
# globbed: the host owns things beside them, and a glob would take those too.
TRACKED_DIRS=(backend ui addons openspec)
TRACKED_FILES=(docker-compose.yml .env.example Makefile)

echo "==> Staging $(git rev-parse --short HEAD) on ${TARGET}"
git archive --format=tar HEAD | ssh "${TARGET}" "
  set -euo pipefail
  rm -rf '${STAGING}' && mkdir -p '${STAGING}'
  tar -x -C '${STAGING}'
  # AppleDouble residue from a macOS tar reaches the Docker build context and
  # has, once, made a build cache a stale layer.
  find '${STAGING}' -name '._*' -delete
"

# BEFORE the mirror, because the mirror is what removes the residue. A check
# that runs after can only ever confirm its own work — the first version of this
# script did exactly that, passed cheerfully with a reintroduced dead route
# sitting on the target, and was the same vacuous guard it exists to prevent.
#
# This one reports what the PREVIOUS deploy left, which is the finding: it is
# the line that would have said `/system/hardware-sync` was still there.
echo "==> What the last deploy left behind"
STALE=$(ssh "${TARGET}" "
  [ -d '${REMOTE_DIR}' ] || exit 0
  cd '${STAGING}'
  for d in ${TRACKED_DIRS[*]}; do
    [ -e \"\${d}\" ] && [ -e '${REMOTE_DIR}/'\"\${d}\" ] || continue
    diff -rq --exclude=node_modules --exclude=.next --exclude=data \
      \"\${d}\" '${REMOTE_DIR}/'\"\${d}\" 2>/dev/null | grep '^Only in ${REMOTE_DIR}' || true
  done
" | grep -v '^$' || true)
if [ -n "${STALE}" ]; then
  echo "${STALE}" | sed 's/^/    /'
  echo "    ^ present on the target and not in this commit. The mirror below removes them."
  echo "    If any of these are source files, a previous deploy was shipping deleted code."
else
  echo "    nothing"
fi

echo "==> Mirroring into ${REMOTE_DIR}"
ssh "${TARGET}" "
  set -euo pipefail
  mkdir -p '${REMOTE_DIR}'
  cd '${REMOTE_DIR}'
  for d in ${TRACKED_DIRS[*]}; do rm -rf \"\${d}\"; done
  for f in ${TRACKED_FILES[*]}; do rm -f \"\${f}\"; done
  for d in ${TRACKED_DIRS[*]}; do
    [ -e '${STAGING}/'\"\${d}\" ] && cp -a '${STAGING}/'\"\${d}\" '${REMOTE_DIR}/'
  done
  for f in ${TRACKED_FILES[*]}; do
    [ -e '${STAGING}/'\"\${f}\" ] && cp -a '${STAGING}/'\"\${f}\" '${REMOTE_DIR}/'
  done
  find '${REMOTE_DIR}' -name '._*' -delete
"

# And after, that the mirror did what it said. Weaker than the check above — it
# confirms this script's own work rather than the last deploy's — but it is the
# thing that fails if a remove or a copy silently did not happen.
echo "==> Verifying the target now matches this commit"
RESIDUE=$(ssh "${TARGET}" "
  cd '${STAGING}'
  for d in ${TRACKED_DIRS[*]}; do
    [ -e \"\${d}\" ] || continue
    diff -rq --exclude=node_modules --exclude=.next --exclude=data \
      \"\${d}\" '${REMOTE_DIR}/'\"\${d}\" 2>&1 || true
  done
" | grep -v '^$' || true)

ssh "${TARGET}" "rm -rf '${STAGING}'"

if [ -n "${RESIDUE}" ]; then
  echo "FAILED: the target does not match this commit." >&2
  echo "${RESIDUE}" >&2
  echo >&2
  echo "A 'Only in ${REMOTE_DIR}/...' line is a file this commit deleted and the" >&2
  echo "deployment still has. Rebuilding now would ship it — which is how a" >&2
  echo "removed route kept answering 200 after the commit that removed it." >&2
  exit 1
fi

echo "==> ${REMOTE_DIR} matches $(git rev-parse --short HEAD). Rebuild with:"
echo "    ssh ${TARGET} 'cd ${REMOTE_DIR} && docker compose build backend ui && docker compose up -d'"
