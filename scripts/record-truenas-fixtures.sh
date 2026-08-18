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
usage: scripts/record-truenas-fixtures.sh [--write]

  Reads TRUENAS_URL and TRUENAS_API_KEY from ./.env (or the environment) and
  records what that target answers into addons/contract/truenas_observed.json.

  TRUENAS_VERIFY_TLS is honoured exactly as the add-on honours it, and defaults
  to true here as it does there. This probe authenticates with the API key, so
  turning it off hands that key to whatever answers for the NAS's address.

  --write   also record the WRITE rules, by creating and deleting a throwaway
            account (syndra-fixture-probe). Without it only reads are recorded,
            and the write rules keep whatever was recorded last.

  The output names the release it came from. A fixture from one major says
  nothing about another, and pretending otherwise is how this went wrong twice.
EOF
  exit 2
}

WRITE=no
case "${1:-}" in
  --write) WRITE=yes ;;
  "") ;;
  *) usage ;;
esac

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

OUT="addons/contract/truenas_observed.json"
echo "==> recording from $TRUENAS_URL (write probes: $WRITE)"

docker run --rm -i \
  -e TRUENAS_URL -e TRUENAS_API_KEY -e WRITE="$WRITE" \
  -e TRUENAS_VERIFY_TLS="${TRUENAS_VERIFY_TLS:-true}" \
  --entrypoint sh python:3.12-alpine -c \
  'pip -q install websockets >/dev/null 2>&1 && python - ' < scripts/lib/record-truenas.py > "$OUT.tmp"

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
echo "Commit it. `addons/truenas/truenas_rules_test.go` asserts the add-on's"
echo "payloads against these refusals, so a release that changes a rule fails the"
echo "suite instead of the deployment."
