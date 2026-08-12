#!/bin/bash
# Check an add-on's bring-up, one leg at a time, in the order the legs fail.
#
# Three separate connections carry an add-on, and every one of them fails in a
# way that looks like the others:
#
#   1. the secret file exists and both containers can read it
#   2. backend <-> add-on: the derived key the backend pins, and the key the
#      add-on serves
#   3. add-on -> the target itself (TrueNAS), which is the only leg needing
#      hardware and the only one this script cannot check for you
#
# Diagnosing 2 and 3 together is what the bring-up sequencing exists to avoid.
# So each check names which leg it is about and stops at the first one that is
# broken — a later check passing against a broken earlier one is not evidence.
set -euo pipefail

cd "$(dirname "$0")/.."

TARGET="${1:?usage: scripts/smoke-test-addon.sh <target>   (e.g. truenas)}"
COMPOSE="${COMPOSE:-docker compose}"
SERVICE="${TARGET}-addon"
SECRET_DIR="${ADDON_TRANSPORT_SECRETS_DIR:-./secrets/addon}"
SECRET="$SECRET_DIR/$TARGET.key"

pass() { printf '  ok    %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1" >&2; shift; printf '        %s\n' "$@" >&2; exit 1; }

echo "==> 1. the secret"

if [ -d "$SECRET" ]; then
  fail "$SECRET is a directory" \
    "Docker created it by bind-mounting a secret that did not exist yet." \
    "Stop the add-on, run: rmdir '$SECRET', then: sudo scripts/gen-addon-secret.sh $TARGET"
fi
[ -f "$SECRET" ] || fail "no secret at $SECRET" \
  "Mint it: sudo scripts/gen-addon-secret.sh $TARGET"

# 0640 root:65532 — owner read for the backend, group read for the add-on's uid.
# A 0600 root-owned file is the failure that looks like a wrong secret: the
# add-on cannot open it, exits, and every backend call then fails to connect.
MODE=$(stat -c '%a' "$SECRET" 2>/dev/null || stat -f '%Lp' "$SECRET")
OWNER=$(stat -c '%u:%g' "$SECRET" 2>/dev/null || stat -f '%u:%g' "$SECRET")
case "$MODE" in
  640) pass "mode 0640" ;;
  600) fail "mode 0$MODE — the add-on runs as uid 65532 and cannot read this" \
        "sudo chmod 0640 '$SECRET' && sudo chown 0:65532 '$SECRET'" ;;
  *)   fail "mode 0$MODE" \
        "0644 or wider makes the deployment's most sensitive value world-readable." \
        "sudo chmod 0640 '$SECRET' && sudo chown 0:65532 '$SECRET'" ;;
esac
[ "$OWNER" = "0:65532" ] && pass "owned root:65532" || \
  fail "owned $OWNER, expected 0:65532" "sudo chown 0:65532 '$SECRET'"

echo "==> 2. backend <-> add-on"

# The two keys, from the two startup logs. The backend logs the key it PINS at
# registration and the add-on logs the key it SERVES at boot; a pin failure
# names three causes it cannot tell apart, and comparing these two hex strings
# is what separates them.
BACKEND_LOG=$($COMPOSE logs backend 2>/dev/null || true)
ADDON_LOG=$($COMPOSE logs "$SERVICE" 2>/dev/null || true)

[ -n "$ADDON_LOG" ] || fail "$SERVICE has no logs — is it running?" \
  "$COMPOSE --profile $TARGET up -d $SERVICE"

REGISTERED=$(printf '%s\n' "$BACKEND_LOG" | grep -F "[ADDON] Registered target=$TARGET" | tail -1 || true)
if [ -z "$REGISTERED" ]; then
  REFUSAL=$(printf '%s\n' "$BACKEND_LOG" | grep -F "[ADDON] $TARGET" | tail -1 || true)
  fail "the backend did not register $TARGET" \
    "${REFUSAL:-No [ADDON] line mentions it at all — is it in ADDON_TARGETS, and did the backend restart since?}" \
    "Registration is read once at startup: $COMPOSE up -d --force-recreate backend"
fi
pass "registered"

# Anchored on the key's LENGTH (32 bytes, 64 hex) rather than on end-of-line:
# `docker compose logs` prefixes every line with a service name and may carry a
# carriage return, and an anchor that a log format can break is an anchor that
# reports "could not check" on a deployment that is fine.
PINNED=$(printf '%s\n' "$REGISTERED" | grep -oE 'pinned_key=[0-9a-f]{64}' | tail -1 | cut -d= -f2 || true)
SERVED=$(printf '%s\n' "$ADDON_LOG" | grep -F '[STARTUP]' | grep -oE '\bkey=[0-9a-f]{64}' | tail -1 | cut -d= -f2 || true)

if [ -z "$PINNED" ] || [ -z "$SERVED" ]; then
  # Not a failure of the transport — a failure to observe it. Said plainly,
  # because "could not check" reported as "ok" is how a smoke test becomes
  # evidence for something it never tested.
  echo "  ----  could not read both startup keys (backend=${PINNED:-?} addon=${SERVED:-?})"
  echo "        Logs may have rotated past them. Recreate both and re-run:"
  echo "        $COMPOSE up -d --force-recreate backend $SERVICE"
elif [ "$PINNED" = "$SERVED" ]; then
  pass "the key the backend pins is the key the add-on serves"
else
  fail "the two ends derived different keys" \
    "backend pins  $PINNED" \
    "addon serves  $SERVED" \
    "Same secret, different salt, or different secrets. Check ADDON_TARGET on" \
    "the add-on equals '$TARGET', then that both read the same file."
fi

# Registration says the deployment is configured; the manifest read is the first
# thing that proves the channel works end to end.
MANIFEST=$(printf '%s\n' "$BACKEND_LOG" | grep -F "$TARGET" | grep -iE "manifest|capabilit" | tail -1 || true)
# An `if`, not `[ ... ] && echo`: under `set -e` that idiom exits the script
# when the test fails if it is ever the last command in its block, and "no
# manifest line yet" is the NORMAL state at a first bring-up.
if [ -n "$MANIFEST" ]; then
  echo "  note  $MANIFEST"
fi

echo "==> 3. $TARGET itself"
echo "  ----  not checked here. This leg needs the real target, and it is the"
echo "        one the bring-up is for: open the target's page, or"
echo "        $COMPOSE logs $SERVICE | grep -i $TARGET"
echo
echo "Legs 1 and 2 are the deployment's own. If they pass and the target page"
echo "still reports trouble, the remaining leg is $TARGET, not Syndra."
