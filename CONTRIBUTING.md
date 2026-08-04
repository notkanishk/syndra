# Contributing to Syndra

Thanks for looking. Syndra mediates access control for real infrastructure, so
the bar here leans toward "explain why" rather than "ship fast" — but that
applies to the reasoning, not the paperwork. Small, clear changes are very
welcome.

## Getting set up

Requires Docker, Go 1.26+, and [Bun](https://bun.sh). Versions are pinned in
[`.tool-versions`](.tool-versions).

```bash
cp .env.example .env
docker compose up -d postgres redis
make dev
```

No Zitadel instance is needed to develop. With `ZITADEL_DOMAIN` unset, Syndra
seeds a demo catalog and accepts a demo session cookie, so every screen is
reachable offline. Confirm which mode you are in from the startup log:
`[DIRECTORY] Source=demo` or `Source=zitadel`.

## Before you open a pull request

```bash
make test     # backend (go test ./...) + UI (vitest)
make lint     # go vet + next lint
```

For changes in `sync/`, additionally:

```bash
cd sync && go test ./... && go vet ./...
```

## What a good change looks like

**Specs move with code.** This repository uses [OpenSpec](openspec/INDEX.md):
intended behaviour is written down, and the spec is the authority on what the
code is *supposed* to do. Any behavioural or contract change should update the
relevant files under `openspec/changes/` in the same PR. A typo fix does not
need a spec revision; a new endpoint, a changed response shape, or an altered
failure mode does.

If you find code and spec disagreeing, that is worth an issue on its own — we
would rather know.

**Tests exercise the failure, not just the success.** The interesting test is
the one that fails when the logic breaks. A test that only walks the happy path
mostly documents that the function exists.

**Fix causes, not symptoms.** If a bug report names one call site, check the
others before patching. A guard in the shared function is a smaller change than
a guard in each caller, and it is the one that actually fixes the bug.

**Match the surrounding code.** Conventions worth knowing:

- Design tokens live only in `ui/src/app/globals.css`. Never a hardcoded colour
  in a component; both themes are authored in full.
- Navigation structure lives only in `ui/src/lib/nav.ts`.
- Token shaping lives only in `backend/internal/claims`, applied on read by both
  the Actions v2 handler and the simulator. A preview computed by different code
  from the token it previews is a preview of nothing.
- Mutation endpoints decode strictly (`decodeJSONStrict`).
- The backend is the single mutation authority. The frontend signals intent; the
  backend decides.
- Syndra-mediated Zitadel mutations leave a trace *before* the Management API
  call. That is what makes drift detectable rather than invisible.

## Commit messages

Conventional commits — `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`.
Explain *why* in the body when the reason is not obvious from the diff. The
subject line describes the change; the body describes the problem it solves.

## Reporting bugs

Use the issue templates. The single most useful thing you can include is what
you expected versus what happened, plus whether you were in demo mode or against
a live Zitadel — the two behave deliberately differently, and knowing which
halves the search.

**Do not report security vulnerabilities in a public issue.** See
[`SECURITY.md`](SECURITY.md).

## A note on scope

Syndra is built for one makerspace and generalized only where generalizing was
free. If you want to use it somewhere else, that is great — but a change that
adds configuration surface for a case nobody has yet is likely to be declined in
favour of one that solves the case you actually have. Bring the use case.
