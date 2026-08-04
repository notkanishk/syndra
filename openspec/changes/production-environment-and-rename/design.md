> **Status:** In Progress | [< Index](../../INDEX.md) · [Architecture](../syndra-core-architecture/design.md) · [DEPLOY.md](../../../DEPLOY.md)

# Design — Production Environment & Rename to Syndra

**Phase:** 5.5 (operational)
**Related:** [zitadel-actions-v2-deployment](../zitadel-actions-v2-deployment/design.md) (target registration), [wave-2-part-3-operational-polish](../wave-2-part-3-operational-polish/design.md) (deployment surface cleanup)

## 1. Why a second host and not a second project

Zitadel Actions v2 target management lives at the **instance scope**
(`zitadel/actions/README.md` §Service-Account Permissions). `zitadel/actions/targets.json`
binds executions on `function: preaccesstoken`, `preuserinfo`, and seven
`event:` conditions — none of which accept an org or project filter.

Two consequences, both fatal to a shared instance:

1. `register.sh` upserts targets **by name**. Registering production rewrites the
   existing target's endpoint, and the other deployment stops receiving claims
   with nothing logged on either side.
2. Renaming targets to avoid the collision is worse: both fire on every event in
   the instance, so the development backend receives production's
   `user.grant.added`, creates onboarding intents, and writes back to production.

A read-only service account for development does not help. It addresses the
Management API surface, not Actions dispatch, and Syndra's purpose *is* mutating
Zitadel — a read-only instance of it cannot exercise grant issuance, onboarding,
or bundle cascade, which is most of what needs testing.

So: production keeps the existing instance (`auth.example.org`), which
already holds the real users and the Google Workspace IdP. Development moves to a
new instance in a follow-up change. The migration cost lands on the throwaway
environment.

## 2. Secrets

`scripts/gen-prod-env.sh` runs once on the host and writes `.env` at mode 600
with `openssl rand -hex 32` values for `POSTGRES_PASSWORD`, `SYNDRA_API_KEY`, and
`SESSION_SECRET`.

It refuses to overwrite an existing `.env`. A regenerated `POSTGRES_PASSWORD`
does not match an already-initialized Postgres volume, so a careless re-run would
lock the backend out of its own data while appearing to succeed. Replacing one is
a deliberate manual act.

`.env.example` stays the development reference and now says so. Its values are
shared literals; hand-adapting it for production is how the old host ended up
with `mkauth_secure_password` in a live deployment.

### Why `POSTGRES_PASSWORD` is required rather than defaulted

Compose interpolation supports nesting, so `DB_DSN` defaults to
`postgres://syndra:${POSTGRES_PASSWORD}@postgres:5432/syndradb?sslmode=disable`.
The credential has exactly one definition, and a missing value is a startup error
instead of a silently shared secret. Verified: `docker compose config` fails
without it.

## 3. Deploy path

A self-hosted GitHub Actions runner (label `syndra-prod`) runs as an
unprivileged `runner` user inside the production LXC. The workflow drives the
long-lived clone at `/opt/syndra` rather than its own workspace, because that
directory also holds `.env`, the Zitadel service-account key, and the identity of
the named Docker volumes.

**Accepted risk:** anyone who can push to `main` gets arbitrary code execution in
production, because the workflow file is itself part of the pushed code. Mitigated
by branch protection on `main`, not by anything in this repo. Recorded here so it
is a decision rather than an oversight.

### The reachability assertion

The old host's `MKAUTH_EXTERNAL_URL` pointed at `198.51.100.14` long after the box
moved to `.16`. Zitadel kept POSTing to an address that did not answer, so the
claim injector and event listener were dead for months. The failure lived
entirely on the caller's side of the wire — nothing in any Syndra log could have
shown it.

Every deploy now POSTs an unsigned request to the configured
`SYNDRA_EXTERNAL_URL` and fails on connection failure. Reaching the handler and
being rejected is the **passing** case: the check asks whether the path exists,
not whether the request is valid.

## 4. The rename

1841 occurrences over four casings. Mechanical except two.

**`drift_items.drift_type`** is stored data under a CHECK constraint, so
`'mkauth_only'` could not be text-replaced. Migration `000025` drops the
constraint, updates the rows, then re-adds it — in that order, because either
constraint alone forbids the other's data.

**`mkauth_roles` / `only_in_mkauth`** are reconciliation wire fields consumed by
`ui/src/components/review/UnexplainedAccess.tsx`, so they move in lockstep with
the UI.

`backend/db/migrations/000016_drift_queue.up.sql` keeps `'mkauth_only'` in its
CHECK. An applied migration records what the schema was; editing it would make
the recorded history disagree with every database that ran it.

### The guard test had to change too

`TestDriftMigrationEnumsMatchCode` read `000016` alone. After `000025` it would
still pass while the live schema disagreed with the code — the exact failure the
guard exists to prevent. It now concatenates every `*.up.sql`, so a constraint
moving between migrations does not blind it.

## 5. Verification

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd sync    && go build ./... && go vet ./... && go test ./...
cd ui      && bun run test && bun run lint && bun run build

# compose contract
POSTGRES_PASSWORD=x docker compose config --services   # must omit sync
docker compose config                                   # must fail: password required
```

Result at time of writing: backend and sync clean; UI 349 tests, lint clean,
production build succeeds.

## 6. Follow-ups

- Second Zitadel instance for development; repoint `198.51.100.16`.
- Rebuild `198.51.100.16` from `DEPLOY.md` rather than repairing it in place.
- `scripts/deploy-lxc.sh` still targets `198.51.100.14` and `rm -rf`s the remote
  directory before extracting, deleting `.env` and the machine key with it.
- GitHub repository rename to `syndra`; update `origin` afterwards.
