> **Status:** Complete | Phase 5 | [< Index](../../INDEX.md)

## Why

A deployment running against a live Zitadel was showing demo bundles, demo rules and demo grants beside real ones, with nothing on screen saying so. Two separate defects put it there and a third kept it hidden.

**Compose forced seeding on.** `docker-compose.yml` declared `SYNDRA_SEED_DEMO=${SYNDRA_SEED_DEMO:-true}`. The backend already decides this correctly on its own — seed when no live directory came up, don't when one did — and the compose default overrode that decision for every deployment whose `.env` simply didn't mention the variable. A production stack seeded fixtures on first boot because of a default in a file nobody re-reads.

**Turning it off does not undo it.** `SYNDRA_SEED_DEMO=false` stops the seeder writing. It removes nothing. The rows an earlier run wrote stay in Postgres and go on being served, and there was no supported way to remove them — no script, no target, no documented statement of which tables the seeder touches.

**The signal keyed off the wrong thing.** `seed_active` reports whether *this process* seeded. The banner that warned about demo-data-under-a-live-directory read that flag, so an operator who noticed the problem, set the variable to false and restarted saw the banner disappear — which reads as confirmation the fix worked — while every seeded row remained on screen. The one signal that existed actively misled at exactly the moment it was consulted.

Separately, the console had lost its operator instructions. The Identity provider page carried an Action signing key stat card and nothing else; the pre-rebuild console had shown the rotate command with a copy button, and the backend has been returning `rotate_command` in its response the whole time with no consumer. Rotation is a two-part operation — Zitadel mints the key, the backend must restart holding it — and the half that gets forgotten is the second one. The same gap exists for the sync container, which refuses to start without LLDAP credentials and therefore restart-loops forever on any deployment without an LDAP server, with no page in the product acknowledging it.

Targets **Phase 5** (operator experience). Follows `ia-screen-completion`.

## What Changes

**Residue, not seeding, is the signal.** `demo.ProjectIDs()` and `demo.UserIDs()` expose the seeder's fingerprint; `db.CountDemoResidue` counts stored rows referencing either, across all eight tables the seeder writes. `GET /system/mode` reports it as `seed_residue` alongside a `reset_command`. The banner keys off the count, so it survives the restart that used to silence it, and states the number.

**A supported reset.** `scripts/reset-data.sh` in two modes: `demo` removes only rows referencing a fixture, leaving real people, real projects and every operator decision intact; `all` truncates every operator table for a genuine blank slate. Both are dry-run by default, print per-table counts, require typed confirmation to commit, and flush the derived claim cache. Neither touches Zitadel. `demo` mode additionally names any real account that loses access because it holds a demo bundle or a grant on a demo project — those rows cascade out with the fixture and would otherwise vanish without appearing in any count.

**The compose default is removed,** so the backend's own auto-detection governs again.

**Commands the operator runs, on the screens where they'd look.** A shared `CommandBlock` renders a copyable command with the steps that must follow it. Identity provider regains the rotate command (or the register command when no key is installed, because there is nothing to rotate), the env swap, the restart, the verification, and a statement of what happens during the swap window. Hardware sync explains the restart-looping sync container. The demo-residue banner carries the reset command inline.

None of these becomes a button. Each has a failure mode where the console reports success and the system is in a half-state — Zitadel holding a key the backend is not verifying against, a reset committed against the wrong database. An operator in a terminal has the exit code, the stderr, and the ability to stop halfway; a spinner has none of that.

## Impact

- **Affected specs**: operational-readiness
- **Affected code**: `backend/internal/{demo,db,handlers}`, `ui/src/components/{ui,states}`, `ui/src/app/{zitadel,system/hardware-sync}`, `scripts/reset-data.sh`, `Makefile`, `docker-compose.yml`, `.env.example`
- **Behaviour change**: `SYNDRA_SEED_DEMO` no longer defaults to `true` under Compose. A deployment that relied on that default to get demo data now needs it set explicitly. `GET /system/mode` runs one extra counting query per call (60s UI poll); a failure logs and reports zero rather than failing the probe.
