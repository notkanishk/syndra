#!/bin/bash
# Record what a REAL TrueNAS answers, into the contract fixtures.
#
# Every serious defect this add-on has shipped had the same shape: a fixture
# somebody wrote by hand, agreeing with the code that read it, disagreeing with
# the target. Two of them cost a day each.
#
#   `system.version` answers `TrueNAS-25.10.5`; the fixture said `25.04.2.1`,
#   so the version gate refused every mutation on a supported release.
#
#   `user.create` REFUSES a payload with no password; the fixture answered it
#   with a success, so account creation had never worked against any release
#   and both suites were green.
#
# A fixture nobody can write by hand cannot drift that way. This asks the target
# and writes down what it said — including its REFUSALS, which are the half that
# matters and the half no hand-written fixture ever contains.
#
# READ-ONLY BY DEFAULT. The probe set that requires writing creates a throwaway
# account and deletes it again; it runs only with --write, and refuses to touch
# a name that already exists.
set -euo pipefail

cd "$(dirname "$0")/.."

usage() {
  cat >&2 <<'EOF'
usage: scripts/record-truenas-fixtures.sh [--write | --methods]

  Reads TRUENAS_URL and TRUENAS_API_KEY from ./.env (or the environment) and
  records what that target answers into addons/contract/truenas_observed.json.

  TRUENAS_VERIFY_TLS is honoured exactly as the add-on honours it, and defaults
  to true here as it does there. This probe authenticates with the API key, so
  turning it off hands that key to whatever answers for the NAS's address.

  --write   also record the WRITE rules, by creating and deleting a throwaway
            account (syndra-fixture-probe). Without it only reads are recorded,
            and the write rules keep whatever was recorded last.

  --methods the version check's baseline (design-version-check.md §4.1,
            slice 9a): reads system.version and core.get_methods only, and
            writes addons/truenas/compat/baseline/TrueNAS-<version>.methods.json
            and nothing else. Asks at a silent prompt for the elevated key, for
            user.delete's entry; press Enter to leave it out. Prints no address
            and no key. Refuses --write.

  The output names the release it came from. A fixture from one major says
  nothing about another, and pretending otherwise is how this went wrong twice.
EOF
  exit 2
}

WRITE=no
METHODS=no
for arg in "$@"; do
  case "$arg" in
    --write) WRITE=yes ;;
    --methods) METHODS=yes ;;
    *) usage ;;
  esac
done
if [ "$METHODS" = yes ] && [ "$WRITE" = yes ]; then
  echo "error: --methods refuses --write: no account is created on the way to a schema" >&2
  exit 2
fi

[ -f .env ] && set -a && . ./.env && set +a
: "${TRUENAS_URL:?TRUENAS_URL is required (wss://host/api/current)}"
: "${TRUENAS_API_KEY:?TRUENAS_API_KEY is required}"

case "$TRUENAS_URL" in
  wss://*) ;;
  *) echo "error: TRUENAS_URL must be wss://. TrueNAS REVOKES a user-linked API key" >&2
     echo "       presented over plaintext — a ws:// probe destroys the credential." >&2
     exit 1 ;;
esac

command -v docker >/dev/null || { echo "error: docker is required (the probe runs in a container)" >&2; exit 1; }

# Pinned, because the key is in the container's environment: the image by
# digest, websockets by version and hash (any of its files pip may pick).
IMAGE="python:3.12.10-alpine3.21@sha256:9c51ecce261773a684c8345b2d4673700055c513b4d54bc0719337d3e4ee552e"
PIP='printf "%s\n" "websockets==15.0.1 --hash=sha256:3be571a8b5afed347da347bfcf27ba12b069d9d7f42cb8c7028b5e98bbb12597 --hash=sha256:d5f6b181bb38171a8ad1d6aa58a67a6aa9d4b38d0f8c5f496b9e42561dfc62fe --hash=sha256:f7a866fbc1e97b5c617ee4116daaa09b722101d4a3c170c787450ba409f9736f --hash=sha256:82544de02076bafba038ce055ee6412d68da13ab47f0c60cab827346de828dee" > /tmp/req.txt && pip -q install --disable-pip-version-check --require-hashes --no-deps -r /tmp/req.txt >/dev/null'

if [ "$METHODS" = yes ]; then
  # The elevated key: pasted once, silent, held in this shell for one run and
  # handed to the container on stdin, never in its environment, a file or a log.
  ELEVATED=""
  if [ -t 0 ]; then
    read -rsp "Elevated key for user.delete (Enter to leave it out): " ELEVATED; echo >&2
  else
    read -r ELEVATED || true
  fi
  echo "==> recording the methods baseline (the address is not printed)" >&2
  TMP="$(mktemp)"
  trap 'rm -f "$TMP"' EXIT
  printf '%s\n' "$ELEVATED" | docker run --rm -i \
    -e TRUENAS_URL -e TRUENAS_API_KEY \
    -e TRUENAS_VERIFY_TLS="${TRUENAS_VERIFY_TLS:-true}" \
    -v "$PWD/scripts/lib/record-truenas-methods.py:/r/methods.py:ro" \
    -v "$PWD/addons/truenas/compat_deps.go:/r/compat_deps.go:ro" \
    --entrypoint sh "$IMAGE" -c \
    "$PIP && python /r/methods.py /r/compat_deps.go" > "$TMP" || {
      echo "error: the capture failed; nothing written" >&2; exit 1; }
  ELEVATED=""
  # The version names the file, so it is checked before it is used as a path.
  VERSION="$(python3 -c 'import json,re,sys; v=json.load(open(sys.argv[1]))["version"]; assert re.fullmatch(r"TrueNAS-[0-9][0-9A-Za-z.-]*", v); print(v)' "$TMP")" || {
    echo "error: the capture is not valid; nothing written" >&2; exit 1; }
  OUT="addons/truenas/compat/baseline/$VERSION.methods.json"
  mkdir -p "$(dirname "$OUT")"
  mv "$TMP" "$OUT"
  echo "==> wrote $OUT. Copy back this file only." >&2
  exit 0
fi

OUT="addons/contract/truenas_observed.json"
echo "==> recording from $TRUENAS_URL (write probes: $WRITE)"

docker run --rm -i \
  -e TRUENAS_URL -e TRUENAS_API_KEY -e WRITE="$WRITE" \
  -e PREVIOUS_RECORDING="$(cat "$OUT" 2>/dev/null || echo '{}')" \
  -e TRUENAS_VERIFY_TLS="${TRUENAS_VERIFY_TLS:-true}" \
  --entrypoint sh "$IMAGE" -c \
  "$PIP && python - " < scripts/lib/record-truenas.py > "$OUT.tmp"

# Only replace the recording if the probe produced a parseable one. A half
# written fixture is worse than a stale one: the stale one is at least a
# consistent account of some real release.
python3 -c "import json,sys; json.load(open('$OUT.tmp'))" || {
  echo "error: the probe did not produce valid JSON; keeping the previous recording" >&2
  rm -f "$OUT.tmp"; exit 1; }

mv "$OUT.tmp" "$OUT"
echo "==> wrote $OUT"
python3 - "$OUT" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
print(f"    release  {d.get('product_version')}")
print(f"    reads    {len(d.get('reads', {}))}")
print(f"    refusals {len(d.get('write_rules', []))}")
PY
echo
echo 'Commit it. addons/truenas/truenas_rules_test.go asserts the add-on'\''s'
echo "payloads against these refusals, so a release that changes a rule fails the"
echo "suite instead of the deployment."
