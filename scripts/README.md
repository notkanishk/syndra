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

Staging only (it refuses where `/var/lib/syndra-staging` is absent). Two readers
besides the kit move the add-on's `/health` `target_reads`: its statistics
collector and the backend's 30-second health probe. For one command the add-on
is recreated with `TRUENAS_STATS_COLLECT_EVERY=0` and the backend is stopped;
on any exit both go back and the script checks they did. Nothing is written to
`.env`, so the production setting is never changed, only shadowed for the run.

```bash
# on the Mac: the kit for the staging host (x86_64)
GOOS=linux GOARCH=amd64 go build -C addons/conformance -o /tmp/syndra-addon-conform ./cmd/syndra-addon-conform
scp /tmp/syndra-addon-conform root@<staging>:/tmp/
# on staging, from /opt/syndra (the add-on running: COMPOSE_PROFILES=truenas)
SECRET="$(docker volume inspect -f '{{.Mountpoint}}' syndra_truenas_addon_secret)/secret.key" \
scripts/addon-quiet-run.sh -- sh -c \
  '/tmp/syndra-addon-conform -url "https://$ADDON_IP:8443" -target truenas -secret-file "$SECRET"'
```

`$ADDON_IP` is set by the script, so it is expanded inside the quoted `sh -c`, not by
your shell before the script runs. If the script is killed
outright, `scripts/addon-quiet-run.sh --restore` (or the next deploy) puts it back.
