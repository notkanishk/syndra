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
| `gen-addon-secret.sh` | Mints one add-on's transport secret. Run under `sudo`, once per target |
| `smoke-test-addon.sh` | Checks an add-on's bring-up leg by leg, stopping before the target itself |
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

## gen-addon-secret.sh

One secret per add-on target. Both ends derive both keys from it — the Ed25519
key the add-on serves and the backend pins, and the HMAC key that signs every
request — so there is no certificate to mint, distribute, or renew.

```bash
sudo ./scripts/gen-addon-secret.sh truenas
```

Writes `./secrets/addon/truenas.key` at `0640 root:65532` (owner read for the
backend, group read for the add-on's uid) and prints the `.env` lines naming it.

Run it **before** starting the add-on: Docker creates a *directory* when a bind
mount names a host path that does not exist, and the add-on then exits on a
secret it cannot read.

Re-running for a target that already has one does nothing and exits 0.
Publication is a `link(2)`, which never clobbers, so a file at the destination
is always a complete secret — including after a run killed midway. Rotation is a
deliberate procedure (DEPLOY.md, "Rotating an add-on transport secret"), because
replacing the value under a running pair strands whatever is in flight.

## smoke-test-addon.sh

```bash
./scripts/smoke-test-addon.sh truenas
```

Three connections carry an add-on and each fails looking like the others, so
this checks them in order and stops at the first break:

1. the secret exists, `0640 root:65532`, and is a file rather than the directory
   Docker leaves behind
2. the backend registered the target, and **the key it pins equals the key the
   add-on serves** — diffed from the two startup log lines, which is what
   separates the three causes a pin failure cannot tell apart
3. the target itself — *not* checked here, deliberately. It is the only leg
   needing hardware, and diagnosing it together with leg 2 is what the bring-up
   order exists to prevent.

