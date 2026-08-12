#!/bin/bash
# Mint the transport secret for one add-on target.
#
# ONE value per target. From it both ends derive both keys — the Ed25519 key the
# add-on serves and the backend pins, and the HMAC key that signs every request
# (design `addon-transport-derived-keys`). There is no certificate to mint, no
# CA to keep, and nothing that expires.
#
# ONE file, mounted into both. The scheme is symmetric: both ends hold the same
# bytes by definition, so there is no half either must be kept from, and a
# second copy would only create a state where the two disagree — which is
# exactly the failure this transport exists to remove. The add-on mounts THIS
# target's file alone; the backend mounts the directory, because it holds every
# target.
#
# Run under sudo, once per target. Deliberately NOT called from
# gen-prod-env.sh: that script runs as the unprivileged deploy user, and
# ownership here needs root. It also runs before ADDON_TARGETS exists, so it has
# nothing to iterate.
set -euo pipefail

cd "$(dirname "$0")/.."

usage() {
  cat >&2 <<'EOF'
usage: sudo scripts/gen-addon-secret.sh <target>

  <target>  the add-on's name in the deployment — the entry in the backend's
            ADDON_TARGETS and the add-on's own ADDON_TARGET. The two must be
            identical: the name is the HKDF salt, so a disagreement derives
            different keys from the same secret and fails as a pin mismatch
            that looks exactly like a wrong secret.

env:
  ADDON_TRANSPORT_SECRETS_DIR   where the file lands (default ./secrets/addon)
EOF
  exit 2
}

[ $# -eq 1 ] || usage
TARGET="$1"

# The same shape the backend's splitTargets produces, and no wider. The name
# becomes part of an environment variable name (ADDON_<TARGET>_SECRET), and a
# hyphen there is not merely unconventional: `${ADDON_MY-NAS_SECRET}` is
# Compose's default-value operator, so the variable silently expands to
# something else entirely and the target never registers, with no error anywhere.
if ! printf '%s' "$TARGET" | grep -Eq '^[a-z][a-z0-9_]*$'; then
  echo "error: target name '$TARGET' must match [a-z][a-z0-9_]*" >&2
  echo "       It becomes part of ADDON_<TARGET>_SECRET, where a hyphen is" >&2
  echo "       Compose's default-value operator and would expand to something else." >&2
  exit 1
fi

DIR="${ADDON_TRANSPORT_SECRETS_DIR:-./secrets/addon}"
DEST="$DIR/$TARGET.key"

# The add-on runs as this uid against a read-only mount (addons/truenas/
# Dockerfile). Numeric on purpose — the group need not exist on the host.
ADDON_GID=65532

command -v openssl >/dev/null || { echo "error: openssl not found" >&2; exit 1; }

# --- Preflight ------------------------------------------------------------
#
# Ergonomics, NOT the safety property: it stops a run that cannot succeed
# before anything is created, and names the missing privilege rather than the
# syscall that failed. The no-clobber guarantee comes from the publication step
# below and from nowhere else.

if [ "$(id -u)" -ne 0 ]; then
  echo "error: must run as root" >&2
  echo "       The file is owned root:$ADDON_GID so the backend reads it as owner" >&2
  echo "       and the add-on reads it by group. Re-run with sudo." >&2
  exit 1
fi

if [ -d "$DEST" ]; then
  # Docker creates a DIRECTORY when a bind mount names a host path that does
  # not exist — so this is the footprint of an add-on brought up before its
  # secret was minted. Worth its own message: as a bare "already exists" it
  # reads as "already provisioned", and the operator stops looking.
  echo "error: $DEST is a directory, not a secret" >&2
  echo "       Docker created it by bind-mounting a secret that did not exist yet." >&2
  echo "       Stop the add-on, run: rmdir '$DEST', then re-run this script." >&2
  exit 1
fi

if [ -e "$DEST" ]; then
  # Not a failure. Publication is indivisible, so a file here is a COMPLETE
  # secret — including after a run that was killed between publishing and
  # tidying up. Exit 0: re-running per target is then safe, and an operator who
  # meets this after an interruption can tell "done" from "broken" without
  # reaching for rm on a live secret.
  echo "$TARGET already has a transport secret at $DEST — nothing to do."
  echo "(This is not an error. To rotate deliberately, see DEPLOY.md ->"
  echo " \"Rotating an add-on transport secret\"; it drains first, and moving"
  echo " this file aside without draining strands in-flight mutations.)"
  exit 0
fi

mkdir -p "$DIR"
[ -w "$DIR" ] || { echo "error: $DIR is not writable" >&2; exit 1; }

# --- Mint -----------------------------------------------------------------

# Unique path, restrictive mode from the outset. Unique because a deterministic
# one would let an abandoned temporary collide with the next run's own;
# restrictive from the outset because a run killed before a later chmod would
# otherwise leave key material readable.
umask 077
TMP="$(mktemp "$DIR/.$TARGET.key.XXXXXXXX")"
# Every exit this process still reaches. Not "every failure path" — SIGKILL
# reaches nothing, which is why a stale temporary must be harmless rather than
# prevented, and it is: the DESTINATION decides what happens, and a leftover
# temporary neither changes that nor fails the next run on its own account.
trap 'rm -f "$TMP"' EXIT

# Operator-chosen values are the one input HKDF cannot strengthen.
openssl rand -hex 32 > "$TMP"

# On the temporary, before publication, so the file is never visible at the
# destination with the wrong owner even for an instant.
chown "0:$ADDON_GID" "$TMP"
chmod 0640 "$TMP"

# --- Publish --------------------------------------------------------------
#
# link(2), never rename(2). This is the entire no-clobber guarantee.
#
# POSIX rename REPLACES its destination, so check-then-rename is a TOCTOU with
# a silent loser: two runs that both see nothing both publish, and the later
# destroys a secret the earlier may already have put into service — leaving the
# two ends on different values, which is the split state this design exists to
# prevent. `ln` fails EEXIST instead, and the temporary is already on the
# destination filesystem, so the hard link is valid.
if ! ln "$TMP" "$DEST" 2>/dev/null; then
  echo "error: could not publish $DEST" >&2
  echo "       Something created it between the check above and this link." >&2
  echo "       Nothing was overwritten; the existing secret is intact." >&2
  exit 1
fi
rm -f "$TMP"

UPPER="$(printf '%s' "$TARGET" | tr '[:lower:]' '[:upper:]')"
cat <<EOF
==> minted the transport secret for '$TARGET'

    $DEST  (0640 root:$ADDON_GID)

Configuration — the backend's .env:

  ADDON_TARGETS=$TARGET
  ADDON_${UPPER}_BASE_URL=https://$TARGET-addon:8443
  ADDON_${UPPER}_SECRET_FILE=/run/secrets/addon/$TARGET.key

The add-on's ADDON_TARGET must be exactly '$TARGET' and its ADDON_SECRET_FILE
the same path. Both are already wired in docker-compose.yml for truenas.

Then: docker compose --profile $TARGET up -d
EOF
