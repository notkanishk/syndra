# Production Environment & Rename to Syndra

**Status:** In Progress
**Phase:** 5.5 (operational — no roadmap feature scope)
**Planes:** Bridge (deployment surface only). Control and Data planes are untouched except by the rename.

## Why

There has only ever been one deployment. `<LEGACY_HOST>` began as a test box and
became the thing people use, growing by hand over several months. It is not a
production environment and cannot be made into one in place: its secrets are
world-readable, its Zitadel Action targets point at a decommissioned IP, and it
holds two sets of Postgres volumes because the checkout directory was once
renamed. A second environment built the same way would inherit all of it.

Production therefore gets its own LXC (`syndra`, `<APP_HOST>`), its own
generated secrets, and a deploy path that does not involve a human at an SSH
prompt. `<LEGACY_HOST>` stays as development and gets rebuilt afterwards.

The compose file blocked this on its own. `POSTGRES_PASSWORD` was a literal, so
every deployment that forgot to override it shared one credential. `SESSION_SECRET`
was never passed to the UI, so session cookies were signed with the backend API
key and rotating one silently invalidated every session. `sync` started
unconditionally and crash-looped wherever LLDAP was absent, making a permanently
restarting container normal in `docker ps`.

Separately, the product is named **Syndra**. MkAuth was the working title and it
appeared 1841 times across code, config, specs, and UI copy.

## What changes

- `POSTGRES_PASSWORD` becomes required with no default; `DB_DSN` derives from it.
- `SESSION_SECRET` reaches the UI container.
- `sync` moves behind a compose profile.
- `scripts/gen-prod-env.sh` generates per-host secrets and refuses to overwrite.
- `.github/workflows/deploy-prod.yml` deploys `main` via a self-hosted runner in
  the production LXC, and asserts the configured Action target URL actually
  reaches the deployment.
- `DEPLOY.md` documents bring-up, steady state, and what the first deployment
  got wrong.
- MkAuth → Syndra everywhere, including migration `000025` for the stored
  `drift_type` value.

## Constraint that shapes the topology

Zitadel Actions v2 targets and executions are **instance-scoped**. Only one
Syndra deployment can own `mkauth-claim-injector` / `mkauth-event-listener` per
Zitadel instance — registering a second repoints the first. Separating
environments therefore requires a second Zitadel instance, not a second project
or organization. Development moves to its own instance in a follow-up change.

## Impact

Development (`<LEGACY_HOST>`) breaks on next pull: compose now expects
`POSTGRES_USER: syndra`, so `pg_isready -U syndra` fails against its
`mkauth`-initialized volume and `backend` never passes `depends_on:
service_healthy`. Its `.env` also still uses `MKAUTH_*`. Accepted — that host is
scheduled for rebuild.
