# Syndra — agent orientation

Identity & access orchestration layer for [Zitadel](https://zitadel.com), built
for an academic makerspace. Zitadel is the source of truth; Syndra is the policy
layer on top of it and is never a second source of truth.

If you are Claude Code, read [`CLAUDE.md`](CLAUDE.md) as well — it carries the
same rules plus the tool-specific workflow.

## Read before you write

| Question | Where |
|---|---|
| What is this supposed to do? | [`openspec/INDEX.md`](openspec/INDEX.md) — the spec hub |
| What is already known to be broken or missing? | [`openspec/NEXT.md`](openspec/NEXT.md) |
| How is it structured? | [`openspec/changes/syndra-core-architecture/design.md`](openspec/changes/syndra-core-architecture/design.md) |
| What did the last audit find? | [`docs/AUDIT.md`](docs/AUDIT.md) |
| What does this env var do? | [`.env.example`](.env.example) — every variable documented inline |

**The specs are authoritative over the code.** Where they disagree, that is a bug
in one of them, and saying so is more useful than quietly picking a side.

## Build and test

```bash
cd backend && go test ./... && go vet ./...
cd ui      && bun run test && bun run lint && bun run build
cd sync    && go test ./... && go vet ./...
```

Or `make test` / `make lint` from the root.

Use `bun run test`, never `bun test` — Bun's own runner picks up the same files,
knows nothing about the vitest API they use, and reports ~75 phantom failures.

## Non-negotiable conventions

- **Design tokens only in `ui/src/app/globals.css`.** No hardcoded colour in a
  component, ever. Both themes authored in full.
- **Navigation structure only in `ui/src/lib/nav.ts`.** Structure never moves in
  response to data.
- **Absolute URLs only via `ui/src/lib/request-url.ts`.** Build the origin from
  `x-forwarded-*`; never mutate the incoming URL. Duplicating this logic is how
  it broke before.
- **Token shape only in `backend/internal/claims`**, applied on read by both the
  Actions v2 handler and the simulator. A preview computed by different code
  than the token it previews is a preview of nothing.
- **Strict JSON decoding** (`decodeJSONStrict`) on every mutation endpoint.
- **The backend is the single mutation authority.** The frontend signals; the
  backend decides.
- **Zitadel mutations leave a trace before the API call** — intent ledger for
  direct grants, outbox for cascades. That ordering is what makes drift
  detectable instead of invisible. Do not reorder it.
- **Dependencies are injected via `deps.go`** so things are testable without a
  live database or Zitadel.

## After changing anything

Scale the effort to the change — a typo needs none of this, a behavioural change
needs all of it.

1. **Update the specs.** Create or update the relevant change under
   `openspec/changes/`: `proposal.md`, `tasks.md`, `design.md`, and the affected
   `specs/*.md`. Documented intent should match the code when you are done.
2. **Write tests that fail when the logic breaks** — the failure mode, not just
   the happy path. Run the suites above for every module you touched.
3. **Refresh any code-index tooling** you rely on, so it reflects new, renamed,
   and deleted symbols.

## Two traps worth knowing up front

**Actions v2 is instance-scoped.** Targets and executions belong to the whole
Zitadel instance, not an org or project. A second deployment registering against
the same instance silently repoints the first. A second environment needs a
second *instance*.

**Demo mode and live mode are not a spectrum.** With `ZITADEL_DOMAIN` unset,
Syndra seeds a demo catalog and accepts demo session cookies. With a live Zitadel
configured, demo cookies are rejected and demo entities are never serialized.
Check the startup log for `[DIRECTORY] Source=demo` or `Source=zitadel` before
concluding anything about behaviour.

---

<!-- BEGIN OPENLORE (managed — edits inside this block will be overwritten) -->
<!-- openlore-fingerprint: 25cdd746ebf39b56 -->
This project uses OpenLore for persistent architectural memory.

ALWAYS call `orient()` (via the openlore MCP server, or `bunx openlore orient --json`)
before reading source files when starting a new task. This returns the relevant
functions, callers, spec sections, and insertion points for the task at hand —
one structural lookup instead of file-by-file rediscovery.

OpenLore prefixes tool responses with a brief, factual freshness note (the
Epistemic Lease) once your cached context has aged or the repo has moved since
your last `orient()`. It is informational — re-`orient()` if you are relying on
cached cross-module structure; otherwise carry on.

For the MCP setup, ensure `openlore mcp` is configured as an MCP server.
See https://github.com/clay-good/OpenLore for details.
<!-- END OPENLORE -->
