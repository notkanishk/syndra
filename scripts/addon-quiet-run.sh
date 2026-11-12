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
# environment, so .env keeps the production setting. The restore runs on every
# exit (success, failure, Ctrl-C, a dropped ssh session, a closed output pipe)
# and is verified: the add-on's STATS_COLLECT_EVERY must be the value it had
# before, and the backend must answer /healthz again if it was running before.
# Its outcome also goes to syslog (tag addon-quiet-run), for when the terminal
# that started it is gone. Run it inside tmux all the same. If this script is
# killed outright (SIGKILL, a lost host), run it with --restore.
#
# The command sees ADDON_IP, the add-on's address on its network. Quote the
# command so it expands there, not in your shell before this script runs:
#   scripts/addon-quiet-run.sh -- sh -c 'kit -url "https://$ADDON_IP:8443" ...'
# (scripts/README.md has the whole procedure.)
set -euo pipefail
# A write to a closed pipe is an error to handle, never a silent death that
# skips the restore.
trap '' PIPE

cd "$(dirname "$0")/.."

read -ra COMPOSE <<<"${COMPOSE:-docker compose}"
ADDON=truenas-addon
VAR=TRUENAS_STATS_COLLECT_EVERY
# Test knobs (repoguard/addon_quiet_run_test.go); never set them on a host.
MARKER="${STAGING_MARKER:-/var/lib/syndra-staging}"
ENV_FILE="${SYNDRA_ENV_FILE:-.env}"

# A terminal that hung up (or a reader that went away) is not written to again:
# bash keeps output it failed to write and a later child, a $(…) included,
# would flush it into whatever it writes, the settings check among them.
say() { printf '%s\n' "$*" 2>/dev/null || exec >/dev/null 2>&1; }
die() { printf 'addon-quiet-run: %s\n' "$*" >&2 2>/dev/null || exec >/dev/null 2>&1; exit 1; }
note() { if command -v logger >/dev/null; then logger -t addon-quiet-run -- "$*" || true; fi; }

# Staging only, by two facts production cannot have together: the directory
# production pushes its nightly dump into, and the stand-in Zitadel (staging's
# .env sets ZITADEL_FAKE_WORLD and leaves ZITADEL_DOMAIN empty; production's
# deploy refuses an empty ZITADEL_DOMAIN and the backend refuses both set).
envval() { sed -n "s/^$1=//p" "$ENV_FILE" | tail -n 1; }
[[ -d "$MARKER" && -f "$ENV_FILE" && -n "$(envval ZITADEL_FAKE_WORLD)" && -z "$(envval ZITADEL_DOMAIN)" ]] ||
  die "refusing: this is not the staging host (needs $MARKER and a stand-in Zitadel in $ENV_FILE)"

# The staging override, as _staging.yml layers it, so a recreate never drops
# the backend's stand-in Zitadel.
if [[ -z "${COMPOSE_FILE:-}" && -f docker-compose.staging.yml ]]; then
  export COMPOSE_FILE=docker-compose.yml:docker-compose.staging.yml
fi

container() { "${COMPOSE[@]}" ps -q "$1"; }
running() { [[ -n "$(container "$1")" ]]; }
collect_every() {
  local id
  id="$(container "$ADDON")" && [[ -n "$id" ]] || return 1
  docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$id" | sed -n 's/^STATS_COLLECT_EVERY=//p'
}
addon_up() { "${COMPOSE[@]}" up -d --no-deps --wait "$ADDON" >/dev/null; }
# The add-on as .env describes it: the override removed from the environment,
# so the compose file's own interpolation (.env, then its PT1H default) decides.
restore_addon() { env -u "$VAR" "${COMPOSE[@]}" up -d --no-deps --wait "$ADDON" >/dev/null; }
# Running is not answering: a crash-looping container is listed by `ps`.
backend_answers() {
  local port
  port="$("${COMPOSE[@]}" port backend 8080 | sed 's/.*://')" && [[ -n "$port" ]] &&
    curl -fsS --retry 30 --retry-delay 3 --retry-all-errors "http://localhost:$port/healthz" >/dev/null
}

if [[ "${1:-}" == "--restore" ]]; then
  running "$ADDON" || die "the $ADDON service is not running: nothing to restore"
  restore_addon || die "could not recreate $ADDON"
  "${COMPOSE[@]}" start backend >/dev/null || die "could not start the backend"
  now="$(collect_every || true)"
  [[ -n "$now" && "$now" != 0 ]] || die "STATS_COLLECT_EVERY is still '${now}': is $VAR exported in this shell or set in $ENV_FILE?"
  backend_answers || die "the backend does not answer /healthz"
  say "restored: STATS_COLLECT_EVERY=$now, backend answering"
  note "restored by hand: STATS_COLLECT_EVERY=$now"
  exit 0
fi

[[ "${1:-}" == "--" && $# -ge 2 ]] || die "usage: $0 -- <command> [args...]   |   $0 --restore"
shift

running "$ADDON" || die "the $ADDON service is not running: nothing to quiet"
before="$(collect_every)" || die "could not read the add-on's settings"
[[ -n "$before" ]] || die "the running add-on has no STATS_COLLECT_EVERY (deploy a ref with the statistics collector first)"
[[ "$before" != 0 ]] || die "STATS_COLLECT_EVERY is already 0: run $0 --restore first"
backend_was_running=false
if running backend; then backend_was_running=true; fi

restore() {
  local status=$? ok=true now
  # Nothing may stop the restore: not a failing echo on a terminal that hung
  # up, not a second Ctrl-C while compose waits on the health check.
  set +e
  trap '' HUP INT TERM
  trap - EXIT
  say "==> restoring"
  restore_addon || ok=false
  if $backend_was_running; then "${COMPOSE[@]}" start backend >/dev/null || ok=false; fi
  now="$(collect_every)"
  if [[ "$now" != "$before" ]]; then
    say "  FAIL  STATS_COLLECT_EVERY is '$now', was '$before' before: run $0 --restore"
    ok=false
  else
    say "  ok    STATS_COLLECT_EVERY=$now (as before)"
  fi
  if $backend_was_running; then
    if backend_answers; then
      say "  ok    backend answering"
    else
      say "  FAIL  the backend does not answer /healthz: run $0 --restore"
      ok=false
    fi
  fi
  if $ok; then
    note "restored: STATS_COLLECT_EVERY=$now"
    exit "$status"
  fi
  note "RESTORE FAILED: STATS_COLLECT_EVERY=$now (was $before); run $0 --restore"
  exit 1
}
trap restore EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

say "==> quieting (was STATS_COLLECT_EVERY=$before)"
note "quieting: collector off, backend stopped (was STATS_COLLECT_EVERY=$before)"
if $backend_was_running; then "${COMPOSE[@]}" stop backend >/dev/null; fi
export "$VAR=0"
addon_up
unset "$VAR"
[[ "$(collect_every)" == 0 ]] || die "the add-on did not take STATS_COLLECT_EVERY=0"
say "  ok    collector off, backend stopped"

ADDON_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$(container "$ADDON")" | awk '{print $1}')"
[[ -n "$ADDON_IP" ]] || die "could not read the add-on's address"
export ADDON_IP

say "==> running: $*"
"$@"
