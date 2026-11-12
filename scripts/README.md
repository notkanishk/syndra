# scripts

Operational helpers. Most are also reachable through `make` targets — see the
[`Makefile`](../Makefile).

| Script | What it does |
|---|---|
| `gen-prod-env.sh` | Generates a production `.env` with fresh random secrets at mode 600 |
| `reset-data.sh` | Clears demo rows or truncates every operator table. Dry run by default |
| `smoke-test-action-v2.sh` | Signed POST to `/api/action/inject`, asserts claim injection |
| `smoke-test-event-listener.sh` | Signed POST to `/api/webhooks/zitadel`, asserts event handling |
| `smoke-test-lxc.sh` | Checks UI, API health, and container status on a deployed host |
| `smoke-test-addon.sh` | Checks an add-on's bring-up leg by leg, stopping before the target itself |
| `addon-quiet-run.sh` | Staging only: runs one command (a kit run) with the TrueNAS add-on's collector off and the backend stopped, then restores both and verifies |
| `record-truenas-fixtures.sh` | Records what a real TrueNAS answers — and refuses — into the contract fixtures |
| `lib/` | Shared shell helpers |

## gen-prod-env.sh

Run once, on the production host, from the compose directory.

```bash
EXTERNAL_URL=https://syndra.example.org \
ZITADEL_DOMAIN=auth.example.org \
  scripts/gen-prod-env.sh
```

Both variables are required with no default. A default would be one specific
installation's hostname, which every other operator would then inherit without
noticing — and `EXTERNAL_URL` is the address *Zitadel* calls, so a wrong value
fails silently. No local health check exercises it.

**It refuses to overwrite an existing `.env`.** A regenerated
`POSTGRES_PASSWORD` no longer matches an already-initialized database volume, so
a careless re-run would lock the backend out of its own data while reporting
success. Replacing an `.env` is a deliberate manual act.

## reset-data.sh

Dry run unless you pass `--apply`, which then asks for typed confirmation.

```bash
make reset-demo-data            # show what would go
make reset-demo-data APPLY=1    # remove only rows referencing demo fixtures
make reset-all-data APPLY=1     # truncate every operator table
```

**Neither touches Zitadel.** Clearing Syndra's ledger does not revoke anything
upstream — the next reconciliation sweep re-detects those grants as unexplained
access, which is how they get deliberately re-adopted rather than silently
dropped.

## Smoke tests

The two signed smoke tests need the corresponding signing key in the
environment. They exist because the Actions path is the one place where a
misconfiguration produces no error on the Syndra side at all: Zitadel calls, the
call fails, and nothing local ever notices.

## smoke-test-addon.sh

```bash
./scripts/smoke-test-addon.sh truenas
```

Three connections carry an add-on and each fails looking like the others, so
this checks them in order and stops at the first break:

1. the deployment's own minting service ran, and the secret it left is
   `0640 root:65532` as seen from inside the add-on — the only view that
   matters, since no copy exists on the host
2. the backend registered the target, and **the key it pins equals the key the
   add-on serves** — diffed from the two startup log lines, which is what
   separates the three causes a pin failure cannot tell apart
3. the target itself — *not* checked here, deliberately. It is the only leg
   needing hardware, and diagnosing it together with leg 2 is what the bring-up
   order exists to prevent.


## addon-quiet-run.sh

Staging only. It refuses unless both hold: `/var/lib/syndra-staging` exists, and
`.env` has the stand-in Zitadel (`ZITADEL_FAKE_WORLD` set, `ZITADEL_DOMAIN`
empty). Two readers besides the kit move the add-on's `/health` `target_reads`:
its statistics collector and the backend's 30-second health probe. For one
command the add-on is recreated with `TRUENAS_STATS_COLLECT_EVERY=0` and the
backend is stopped. On any exit, including a dropped ssh session, both go back
and the script checks they did (the add-on's setting equals its value before,
the backend answers `/healthz`). The outcome also goes to syslog
(`journalctl -t addon-quiet-run`). Nothing is written to `.env`, so the
production setting is never changed, only shadowed for the run.

The whole staging run, in order. Every step that reaches the real NAS is the
owner's to approve:

```bash
# Every compose call by hand on staging layers the staging override, or a
# recreated backend loses its stand-in Zitadel and starts on the demo directory:
export COMPOSE_FILE=docker-compose.yml:docker-compose.staging.yml

# 0. Hold merges to main for the run: a main deploy re-checks-out staging,
#    stops the backend and the add-on, and resets the database mid-run.
# 1. Deploy the contract branch to staging (it has the collector; main does not):
gh workflow run deploy-staging.yml -f ref=feat/addon-contract
# 2. On staging, switch the add-on on READ-ONLY: staging shares production's
#    NAS, so nothing staging's backend decides may write there. In
#    /opt/syndra/.env set COMPOSE_PROFILES=truenas, ADDON_TARGETS=truenas and
#    TRUENAS_LIFECYCLE_STATE=read_only, then (from /opt/syndra):
docker compose up -d
# 3. On the Mac: the kit for the staging host (x86_64)
GOOS=linux GOARCH=amd64 go build -C addons/conformance -o /tmp/syndra-addon-conform ./cmd/syndra-addon-conform
scp /tmp/syndra-addon-conform root@<staging>:/tmp/
# 4. On staging, inside tmux, from /opt/syndra. Reads only first; then, only
#    with the owner's OK, the -mutate run, which alone gets --writes (the add-on
#    active for that command, the backend stopped throughout):
SECRET="$(docker volume inspect -f '{{.Mountpoint}}' syndra_truenas_addon_secret)/secret.key"
export SECRET
kit='/tmp/syndra-addon-conform -url "https://$ADDON_IP:8443" -target truenas -secret-file "$SECRET"'
scripts/addon-quiet-run.sh -- sh -c "$kit -json /tmp/conformance-reads.json"
scripts/addon-quiet-run.sh --writes -- sh -c "$kit -mutate -json /tmp/conformance-mutate.json"
# 5. When staging testing is over, staging goes back to its normal settings
#    (no add-on): remove truenas from COMPOSE_PROFILES and ADDON_TARGETS in
#    .env, then
docker compose stop truenas-addon && docker compose up -d
```

`$ADDON_IP` is set by the script, so it is expanded inside the `sh -c` (the kit
line is single-quoted for that), not by your shell before the script runs. The
script layers `docker-compose.staging.yml` itself (as `_staging.yml` does) when
`COMPOSE_FILE` is unset.

If the script is killed outright (SIGKILL, a lost host), run
`scripts/addon-quiet-run.sh --restore`. It recreates the add-on from `.env`,
starts the backend, and fails loudly if the setting is still `0`. A later deploy
also recreates the add-on from `.env`, but only while `COMPOSE_PROFILES`
includes `truenas`. Otherwise `--restore` is the only way back.
