#!/bin/bash
# Run one command (a conformance kit run) against the TrueNAS add-on on
# STAGING while nothing else reads the NAS through it, then put everything
# back exactly as it was.
#
#   scripts/addon-quiet-run.sh -- <command> [args...]
#   scripts/addon-quiet-run.sh --restore        # put it back by hand
#
# Two other readers move the add-on's /health `target_reads` counter, which the
# kit's freshness checks read as the add-on's own reads:
#   - the add-on's statistics collector (STATS_COLLECT_EVERY), and
#   - the backend's 30-second HEALTH probe.
# For the duration of <command> the add-on is recreated with
# TRUENAS_STATS_COLLECT_EVERY=0 and the backend is stopped.
#
# Nothing is written to any file. The override exists only in this process's
# environment, so .env (and with it every later deploy) keeps the production
# setting. The restore runs on every exit, success, failure or Ctrl-C, and is
# verified: the add-on's STATS_COLLECT_EVERY must be the value it had before,
# and the backend must be running again if it was running before. If this
# script is killed outright (SIGKILL, a lost host), run it with --restore, or
# let the next deploy do it (`up -d` recreates the add-on from .env).
#
# The command sees ADDON_IP, the add-on's address on its network. Quote the
# command so it expands there, not in your shell before this script runs:
#   scripts/addon-quiet-run.sh -- sh -c 'kit -url "https://$ADDON_IP:8443" ...'
# (scripts/README.md has the whole kit invocation.)
set -euo pipefail

cd "$(dirname "$0")/.."

read -ra COMPOSE <<<"${COMPOSE:-docker compose}"
ADDON=truenas-addon
VAR=TRUENAS_STATS_COLLECT_EVERY

die() { printf 'addon-quiet-run: %s\n' "$*" >&2; exit 1; }

# Staging only. Production pushes its nightly dump INTO this directory on
# staging, so it exists there and nowhere else; stopping production's backend
# is never what this script is for.
[[ -d "${STAGING_MARKER:-/var/lib/syndra-staging}" ]] ||
  die "refusing: this is not the staging host (no ${STAGING_MARKER:-/var/lib/syndra-staging})"

container() { "${COMPOSE[@]}" ps -q "$1"; }
running() { [[ -n "$(container "$1")" ]]; }
collect_every() {
  docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$(container "$ADDON")" |
    sed -n 's/^STATS_COLLECT_EVERY=//p'
}
addon_up() { "${COMPOSE[@]}" up -d --no-deps --wait "$ADDON" >/dev/null; }

# The add-on as .env describes it: the override removed from the environment,
# so the compose file's own interpolation (.env, then its PT1H default) decides.
restore_addon() { env -u "$VAR" "${COMPOSE[@]}" up -d --no-deps --wait "$ADDON" >/dev/null; }

if [[ "${1:-}" == "--restore" ]]; then
  restore_addon
  "${COMPOSE[@]}" start backend >/dev/null
  printf 'restored: STATS_COLLECT_EVERY=%s, backend running\n' "$(collect_every)"
  exit 0
fi

[[ "${1:-}" == "--" && $# -ge 2 ]] || die "usage: $0 -- <command> [args...]   |   $0 --restore"
shift

running "$ADDON" || die "the $ADDON service is not running: nothing to quiet"
before="$(collect_every)"
[[ -n "$before" ]] || die "could not read STATS_COLLECT_EVERY from the running add-on"
backend_was_running=false
running backend && backend_was_running=true

restore() {
  local status=$? ok=true
  trap - EXIT INT TERM
  echo "==> restoring"
  restore_addon || ok=false
  if $backend_was_running; then "${COMPOSE[@]}" start backend >/dev/null || ok=false; fi
  local now
  now="$(collect_every 2>/dev/null || true)"
  if [[ "$now" != "$before" ]]; then
    printf '  FAIL  STATS_COLLECT_EVERY is %q, was %q before: run %s --restore\n' "$now" "$before" "$0" >&2
    ok=false
  else
    printf '  ok    STATS_COLLECT_EVERY=%s (as before)\n' "$now"
  fi
  if $backend_was_running && ! running backend; then
    printf '  FAIL  backend is not running: run %s --restore\n' "$0" >&2
    ok=false
  elif $backend_was_running; then
    echo "  ok    backend running"
  fi
  $ok || exit 1
  exit "$status"
}
trap restore EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "==> quieting (was STATS_COLLECT_EVERY=$before)"
if $backend_was_running; then "${COMPOSE[@]}" stop backend >/dev/null; fi
export "$VAR=0"
addon_up
unset "$VAR"
[[ "$(collect_every)" == "0" ]] || die "the add-on did not take STATS_COLLECT_EVERY=0"
echo "  ok    collector off, backend stopped"

ADDON_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$(container "$ADDON")" | awk '{print $1}')"
export ADDON_IP

echo "==> running: $*"
"$@"
