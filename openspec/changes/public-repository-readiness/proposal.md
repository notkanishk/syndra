# Public Repository Readiness

**Status:** Complete
**Phase:** 5.5 (operational — no roadmap feature scope)
**Planes:** None. Repository surface, deployment scripts, and documentation only.

## Why

The repository was written for one reader who already knew everything. Going
public changes that: the first thing a person or an agent encounters is the
root directory and a README that did not exist. There was no README, no LICENSE,
no contribution guidance, and no security policy — so an outside reader could not
tell what Syndra is, whether they may use it, or how to report a vulnerability
in something that mediates door access.

Publishing also changes what the repository *discloses*. A history scan found no
secrets in any of 188 commits, but `DEPLOY.md` carried 23 internal addresses and
named which host runs the identity provider, which runs the application, and
which is the legacy box. That is not a leak; it is a wiring diagram of live
access-control infrastructure, published permanently.

Three defects were found while auditing rather than introduced by it, and all
three would have greeted a first-time contributor:

- `make test-ui` ran `bun test` instead of `bun run test`. Bun's own runner
  collects the same files but does not implement the vitest API they are written
  against, so a clean checkout reported **75 failures out of 236 collected** on a
  healthy tree. The real suite is 367 tests, all passing.
- `scripts/gen-prod-env.sh` defaulted `EXTERNAL_URL` and `ZITADEL_DOMAIN` to one
  installation's real hostnames. Any other operator would have generated an
  `.env` that looked complete and pointed at someone else's infrastructure.
  `EXTERNAL_URL` is the address *Zitadel* calls, so a wrong value produces no
  local symptom at all.
- `scripts/smoke-test-lxc.sh` defaulted its host argument to an address
  decommissioned months earlier, so an argument-less run smoke-tested nothing
  and reported success.

## What changes

**Repository surface.** Add `README.md`, `LICENSE` (MIT), `CONTRIBUTING.md`,
`SECURITY.md`, GitHub issue and pull-request templates, and per-module READMEs
for `backend/`, `ui/`, `sync/`, `scripts/`, and `docs/`. Expand `AGENTS.md` from
a tool-configuration stub into real orientation, so an agent arriving with no
context finds the specs, the commands, and the invariants without reading source.

**Disclosure.** Replace host addresses throughout the repository with
placeholders (`<APP_HOST>`, `<PROXY_HOST>`, `<ZITADEL_HOST>`, `<LEGACY_HOST>`)
and real hostnames with `example.org` equivalents. The real values move to
`DEPLOY.local.md`, gitignored. Hostnames are not themselves secret — they appear
in Certificate Transparency logs the moment a certificate issues — but *which
host plays which role* is worth not publishing.

**Root.** Untrack local assistant and index state (`.serena/`,
`.codebase-memory/`, `.codex/`): caches keyed to one checkout's absolute path,
stale for every reader but their author. `CLAUDE.md`, `AGENTS.md`, and the
reusable workflows under `.claude/` stay tracked — those are the artifacts that
make the repository navigable by an agent. Move `AUDIT.md` into `docs/`.

**Defects.** Fix `make test-ui`. Make both script defaults required rather than
inherited. Delete `scripts/deploy-lxc.sh` outright rather than repairing it: the
self-hosted runner supersedes it, and a script that `rm -rf`s a remote directory
holding `.env` and the Zitadel machine key has no safe form worth reaching.

## What does not change

No application behaviour. The backend, the policy engine, the claim shaper, and
the Actions v2 path are untouched. The only executable changes are to the
Makefile and two operational shell scripts, all in the direction of failing
loudly instead of proceeding with a wrong inherited value.

## Risk

Low, with one sharp edge: `gen-prod-env.sh` and `smoke-test-lxc.sh` now fail
without explicitly-set values. That is the intended behaviour — the previous
defaults were silent wrong answers — but any automation invoking them without
arguments will now stop. Nothing in this repository does; `DEPLOY.md` is updated
to pass them explicitly.
