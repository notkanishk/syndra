> **Status:** Public repository delta — repository surface | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: A bootstrap script MUST NOT default to one installation's identity

`scripts/gen-prod-env.sh` MUST treat `EXTERNAL_URL` and `ZITADEL_DOMAIN` as
required, exiting non-zero with a message naming the missing variable. It MUST
NOT supply a default drawn from any particular deployment.

A default here is one installation's hostname inherited by everyone else. The
generated `.env` looks complete, so nothing reports a problem — and
`EXTERNAL_URL` is the address *Zitadel* calls, which no local health check
exercises. The result is a deployment that appears healthy while every Action
posts into someone else's namespace.

The same reasoning applies to any operational script that names a host.
`scripts/smoke-test-lxc.sh` MUST require its host argument rather than defaulting
to one; its previous default outlived the machine it named, so an argument-less
run checked nothing and reported success.

#### Scenario: Generation refuses to proceed on an assumed identity

- **WHEN** `scripts/gen-prod-env.sh` runs with `EXTERNAL_URL` unset
- **THEN** it MUST exit non-zero
- **AND** the message MUST name `EXTERNAL_URL`
- **AND** no `.env` MUST be written

#### Scenario: A smoke test cannot pass without a target

- **WHEN** `scripts/smoke-test-lxc.sh` runs with no argument
- **THEN** it MUST exit non-zero with a usage message
- **AND** it MUST NOT report a passing result

### Requirement: The documented verification command MUST be the one that runs the suite

Every `make` target, README, and contribution instruction that claims to run
tests MUST invoke the runner the tests are written against. For the UI that is
`bun run test` (vitest), never `bun test`.

Bun's own runner collects the same files and executes them against an API they
were not written for. It does not error out — it reports failures. A clean
checkout showed **75 failures across 236 collected tests** while the real suite
was 367 tests, all passing. A verification command that manufactures failures is
worse than none: it trains the reader to distrust the suite, and it is the first
command a new contributor runs.

#### Scenario: The Makefile target reflects the true state of the tree

- **WHEN** `make test-ui` runs against an unmodified checkout
- **THEN** every test MUST pass
- **AND** the count MUST match `cd ui && bun run test`

### Requirement: Published documentation MUST NOT map deployment topology

Documentation in the repository MUST use placeholders for host addresses
(`<APP_HOST>`, `<PROXY_HOST>`, `<ZITADEL_HOST>`, `<LEGACY_HOST>`) and
`example.org` hostnames, with a legend explaining each. Real values MUST live
only in `DEPLOY.local.md`, which MUST be gitignored.

The distinction being drawn is not secrecy but role assignment. The hostnames
themselves become public the moment a certificate issues — Certificate
Transparency is a public log. What the repository can decline to publish is
*which host runs the identity provider*, which runs the application, and which is
the unhardened legacy box, permanently and in one place, for a system that
controls physical access.

#### Scenario: A publication scan finds no topology

- **WHEN** tracked files are scanned for RFC1918 addresses
- **THEN** the only matches MUST be generic CIDR ranges in denylist documentation
- **AND** no tracked file MUST associate a host address with a role

### Requirement: Local index and assistant state MUST NOT be tracked

Caches and machine-specific configuration keyed to one checkout's absolute path
MUST be gitignored: `.openlore/`, `.serena/`, `.codebase-memory/`, `.codex/`.
They are rebuilt from the source tree on demand, and a committed copy is stale
for every reader but its author.

Assistant *documentation* — `CLAUDE.md`, `AGENTS.md`, and the reusable workflows
under `.claude/` — MUST remain tracked. Those are the artifacts that let an agent
arriving with no context find the specs, the commands, and the invariants
without reading source, and they are as much a part of the repository's
navigability as the README.

#### Scenario: A fresh clone carries orientation but no stale state

- **WHEN** the repository is cloned
- **THEN** `README.md`, `CLAUDE.md`, and `AGENTS.md` MUST be present
- **AND** no index or cache directory keyed to another machine's paths MUST be present
