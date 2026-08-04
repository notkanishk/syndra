> **Status:** Complete | [< Index](../../INDEX.md) · [Proposal](proposal.md)

# Tasks — Public Repository Readiness

## 1. Pre-publication audit

- [x] 1.1 Scan all 188 commits for secret file paths — clean (`.env.example` only)
- [x] 1.2 Content-scan history for private key blocks — none
- [x] 1.3 Content-scan HEAD for hardcoded credentials and high-entropy assignments — none
- [x] 1.4 Confirm `zitadel-machine-key.json` is gitignored and in no commit
- [x] 1.5 Enumerate disclosure surface: internal addresses, hostnames, personal emails, absolute home paths

## 2. Disclosure

- [x] 2.1 Replace host addresses with `<APP_HOST>` / `<PROXY_HOST>` / `<ZITADEL_HOST>` / `<LEGACY_HOST>` across 14 files
- [x] 2.2 Replace real hostnames with `syndra.example.org` / `auth.example.org`
- [x] 2.3 Placeholder legend at the top of `DEPLOY.md`; ASCII topology realigned
- [x] 2.4 `DEPLOY.local.md` holds the real substitutions; gitignored
- [x] 2.5 Scrub a third party's email from the design handoff; neutralize the maintainer's institutional address in a test fixture and a doc comment
- [x] 2.6 Replace absolute home paths (`/Users/...`) in historical plan documents with `<repo>`

## 3. Root and tracked-file hygiene

- [x] 3.1 Untrack `.serena/`, `.codebase-memory/`, `.codex/`; add to `.gitignore`
- [x] 3.2 Keep `CLAUDE.md`, `AGENTS.md`, and `.claude/` workflows tracked — these are what make the repo navigable by an agent
- [x] 3.3 Move `AUDIT.md` to `docs/AUDIT.md`
- [x] 3.4 Ignore `*.zip` (design-tool exports) and `DEPLOY.local.md`
- [x] 3.5 Delete stray `.DS_Store` files; `git gc --prune=now` (pack 5.01 MiB → 2.87 MiB, 25 garbage objects cleared)

## 4. Defects found during the audit

- [x] 4.1 `make test-ui` ran `bun test`, not `bun run test` — 75 phantom failures on a clean tree. Fixed; `make test-ui` now reports 367 passing
- [x] 4.2 `gen-prod-env.sh` defaulted to one installation's hostnames — both variables now required
- [x] 4.3 `smoke-test-lxc.sh` defaulted to a decommissioned host — argument now required
- [x] 4.4 Delete `scripts/deploy-lxc.sh` (stale target, `rm -rf`s a remote directory holding `.env` and the machine key)
- [x] 4.5 Update `DEPLOY.md` to invoke `gen-prod-env.sh` with both variables explicitly

## 5. Public documentation

- [x] 5.1 `README.md` — what/why, three-plane architecture, quick start, config, layout, honest status, banner provision
- [x] 5.2 `LICENSE` — MIT
- [x] 5.3 `CONTRIBUTING.md` — setup, verification, spec-moves-with-code, conventions
- [x] 5.4 `SECURITY.md` — private reporting, scope, deliberate design decisions that are not bugs
- [x] 5.5 `.github/ISSUE_TEMPLATE/` (bug, feature, config) and `PULL_REQUEST_TEMPLATE.md`
- [x] 5.6 Module READMEs: `backend/`, `ui/`, `sync/`, `scripts/`, `docs/`
- [x] 5.7 `docs/assets/README.md` — banner instructions
- [x] 5.8 `AGENTS.md` expanded from an OpenLore stub into real orientation, outside the managed block

## 6. Verification

- [x] 6.1 Every relative link in the new documentation resolves — 0 broken
- [x] 6.2 Issue-template YAML parses
- [x] 6.3 `bash -n` on both modified scripts; confirm each fails loudly with a useful message when its required variable is absent
- [x] 6.4 `backend`: test, vet
- [x] 6.5 `sync`: test, vet
- [x] 6.6 `ui`: 367 tests, lint, production build
- [x] 6.7 Re-scan for leftover addresses, hostnames, emails, and absolute paths — clean

## 7. Deferred

- [ ] 7.1 Banner graphic (`docs/assets/banner.png`) — provision left in `README.md`, commented so nothing renders broken until the asset exists
- [ ] 7.2 Screenshots of the console for the README
- [ ] 7.3 Decide whether `install.sh` / `update.sh` survive now that the runner is the deploy path — both still assume a human at an SSH prompt
