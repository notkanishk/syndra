<!--
  BANNER — drop a 1280×320 image at docs/assets/banner.png, then delete this
  comment wrapper so the tag below renders. Kept commented until the asset
  exists so the README never shows a broken image.

<p align="center">
  <img src="docs/assets/banner.png" alt="Syndra" width="100%">
</p>
-->

<h1 align="center">Syndra</h1>

<p align="center">
  <strong>An identity &amp; access orchestration layer for Zitadel.</strong><br>
  Assign access once, in language a human uses. Let the machine work out which
  roles that means, in which projects, and why.
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/backend-Go%201.26-00ADD8">
  <img alt="Next.js" src="https://img.shields.io/badge/frontend-Next.js%20%C2%B7%20Bun-000000">
  <img alt="Status" src="https://img.shields.io/badge/status-running%20in%20production-success">
</p>

---

## What it is

[Zitadel](https://zitadel.com) is very good at being the source of truth for
identity. It is deliberately not opinionated about *policy* — the messy,
organization-specific question of who should get what, and what else that
implies.

Syndra is that opinion, kept outside Zitadel. It sits on top as a control plane:
operators express intent ("this person is a lab supervisor", "printing access
implies door access"), and Syndra resolves that into concrete Zitadel role
grants through the Management API. Zitadel remains authoritative; Syndra never
becomes a second source of truth.

It was built for an academic makerspace, where the same person might be a
student, a paid staff member, and a trained operator of one specific machine —
and where physical door access and digital SSO have to agree with each other.

**Concretely, it gives you:**

- **Access lineage** — for any user, the full picture of what they can reach and
  *why*: granted directly, derived from a bundle, or implied by a mapping rule.
- **Bundles** — named groups of roles across projects. Assign "Lab Supervisor",
  not eleven checkboxes.
- **Mapping rules** — flat, versioned conditionals (`IF printing:user THEN ADD
  door_access:lab_pin`) with cycle detection, instead of a brittle inheritance tree.
- **Claim shaping** — per-application control over the JWT shape, applied to
  real tokens via Zitadel Actions v2, with a simulator that previews the exact
  payload using the same code path that produces it.
- **Expiring grants** — semester-scoped access that actually goes away, swept by
  a background scheduler.
- **Drift detection** — every Syndra-mediated change leaves a trace before the
  Management API call. A Zitadel-side change with no matching trace is surfaced
  for triage rather than silently absorbed.
- **A legacy bridge** — an optional worker reflecting identity into LLDAP, for
  the equipment that still speaks LDAP and nothing else.

## Architecture

Three planes, split by how much thinking each is allowed to do:

```
   ┌──────────────────────────────────────────────────────────────┐
   │  CONTROL PLANE — slow, smart                                 │
   │  Next.js console  ·  Go API  ·  Postgres  ·  policy engine   │
   │  Evaluates rules, then calls the Zitadel Management API       │
   └───────────────────────────┬──────────────────────────────────┘
                               │  grants
                               ▼
                    ┌─────────────────────┐
                    │      ZITADEL        │  ← source of truth
                    └──────────┬──────────┘
                               │  login  ·  Actions v2
   ┌───────────────────────────▼──────────────────────────────────┐
   │  DATA PLANE — fast, dumb                                     │
   │  Redis  ·  precompiled flat roles  ·  claim injection        │
   │  Sits in the token path, so it does no reasoning at all      │
   └──────────────────────────────────────────────────────────────┘

   ┌──────────────────────────────────────────────────────────────┐
   │  BRIDGE PLANE — provisioning                                 │
   │  Go sync worker  →  LLDAP  (Samba, UniFi, door controllers)  │
   │  No exposed ports; acts only on verified backend intents      │
   └──────────────────────────────────────────────────────────────┘
```

The split exists because the token path cannot afford to think. Anything
requiring evaluation happens in the control plane, ahead of time, and lands in
Redis as a flat list the data plane can read without deciding anything.

Full design: [`openspec/changes/syndra-core-architecture/design.md`](openspec/changes/syndra-core-architecture/design.md).

## Quick start

Requires Docker, Go 1.26+, and [Bun](https://bun.sh).

```bash
git clone https://github.com/notkanishk/syndra.git
cd syndra
cp .env.example .env                      # local-dev defaults, no edits needed
docker compose up -d postgres redis       # datastores only
make dev                                  # backend :8080 + UI :3000
```

`make dev` runs the backend and UI directly on your machine, so the two
datastores need to be up first. Migrations run automatically on backend start.

Open <http://localhost:3000>. With no Zitadel configured, Syndra runs in
**local-dev mode**: it seeds a demo catalog of users, projects, and
applications so every screen has something in it, and accepts a demo session
cookie with an Admin/User toggle. No external service is required to explore it.

Point `ZITADEL_DOMAIN` and `ZITADEL_MACHINE_KEY_PATH` at a real instance and the
directory layer flips to live mode — look for `[DIRECTORY] Source=zitadel` in
the startup log. Demo data is bypassed, and demo cookies stop being honoured.

```bash
make test        # backend + UI
make lint
```

## Configuration

Every variable is documented inline in [`.env.example`](.env.example) — what it
does, when to set it, and what breaks if you get it wrong. That file is the
reference; this table is only the orientation.

| Group | Purpose |
|---|---|
| Datastore | Postgres and Redis connections |
| Backend | Port, internal API key, CORS, demo seeding |
| Zitadel OIDC | Operator login via PKCE |
| Zitadel M2M | Management API access for live orchestration |
| Actions v2 | Signing keys for claim injection and the event listener |
| Schedulers | Grant expiry, drift reconciliation, outbox drain |
| Sync / LLDAP | The optional bridge worker |

Do not hand-adapt `.env.example` for production. Run
[`scripts/gen-prod-env.sh`](scripts/gen-prod-env.sh) on the production host — it
mints random secrets at mode 600 rather than inheriting shared literals, and
refuses to overwrite an existing `.env`.

## Repository layout

| Path | What lives there |
|---|---|
| [`backend/`](backend/) | Go API, policy engine, Zitadel client, migrations |
| [`ui/`](ui/) | Next.js console (App Router, Bun) |
| [`sync/`](sync/) | Standalone LLDAP provisioning worker |
| [`zitadel/`](zitadel/) | Actions v2 target manifests and registration scripts |
| [`scripts/`](scripts/) | Env generation, smoke tests, data reset |
| [`openspec/`](openspec/) | Specifications — the authoritative record of intent |
| [`docs/`](docs/) | Design brief, audits, implementation plans |

## Documentation

| Start here | For |
|---|---|
| [`openspec/INDEX.md`](openspec/INDEX.md) | The spec hub — every capability, its status, its spec |
| [`openspec/NEXT.md`](openspec/NEXT.md) | Every open gap and known piece of debt, in one place |
| [`DEPLOY.md`](DEPLOY.md) | Production bring-up and the steady-state deploy loop |
| [`docs/AUDIT.md`](docs/AUDIT.md) | Honest self-assessment: bloat, drift, correctness |
| [`CLAUDE.md`](CLAUDE.md) / [`AGENTS.md`](AGENTS.md) | Orientation for AI assistants working in this repo |

This project uses [OpenSpec](openspec/): behaviour is specified before it is
built, and specs are updated alongside the code that changes them. If a spec and
the code disagree, that is a bug in one of them — please report it.

## Status

Running in production for a single makerspace. Honest maturity by capability:

| Capability | State |
|---|---|
| Role &amp; user management, access governance | Integrated |
| Application claims, token shaping, simulator | Integrated |
| Bundles, mapping rules, cycle detection | Integrated |
| Service catalog, access requests | Integrated |
| Topology graph, audit log | Integrated |
| Lifecycle event propagation, drift triage | Integrated |
| LDAP sync | **Partial** — reconciliation deferred; password compatibility unresolved |
| Provisioning | **Partial** — reconciliation and compensating revocations deferred |

Current gaps are tracked in [`openspec/NEXT.md`](openspec/NEXT.md) rather than
smoothed over here.

**One deployment per Zitadel instance.** Actions v2 targets are instance-scoped,
so a second Syndra registering against the same instance silently repoints the
first. A separate environment needs a separate Zitadel instance — not a separate
project. This surprises everyone once; see [`DEPLOY.md`](DEPLOY.md).

## Contributing

Issues and pull requests are welcome — see [`CONTRIBUTING.md`](CONTRIBUTING.md)
for the workflow, and note that behavioural changes are expected to come with a
spec delta and tests.

## Security

Syndra mediates access control. Please report vulnerabilities privately rather
than in a public issue — see [`SECURITY.md`](SECURITY.md).

## License

[MIT](LICENSE).
