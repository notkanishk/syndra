# Syndra

Identity & Access Management orchestration layer for an academic makerspace. Companion to Zitadel with Google Workspace as sole IdP.

## Quick Navigation

- **Next steps:** `openspec/NEXT.md` — single pickup point: every open gap, operator-gated check, and spec/tooling debt in one place
- **Spec index:** `openspec/INDEX.md` — master hub for all specs, capabilities, and changes
- **Architecture:** `openspec/changes/syndra-core-architecture/design.md` — three-plane design, Zitadel interaction matrix, IdP chain
- **Roadmap:** `openspec/changes/syndra-core-architecture/ROADMAP.md` — phase timeline (1-4 complete, 5-6 ahead)
- **Reality check:** `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` — planned vs integrated
- **UI design system + IA:** `openspec/changes/basic-advanced-ia/design.md` — Basic/Advanced views, the navigation contract, and where the token shape lives

## Tech Stack

- **Backend:** Go, PostgreSQL, Redis (`backend/`)
- **Frontend:** Next.js with Bun runtime (`ui/`)
- **Sync Service:** Go, go-ldap/v3, separate container (`sync/`)
- **Deployment:** Docker Compose in Proxmox LXC

## Build & Test

```bash
cd backend && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
cd sync && go test ./... && go vet ./...
```

## Key Conventions

- **UI:** design tokens live only in `ui/src/app/globals.css`; navigation structure only in `ui/src/lib/nav.ts`. Both themes are authored in full — never a hardcoded hex in a component. Structure never moves in response to data (see `basic-advanced-ia`)
- **Token shape:** `internal/claims` is the only shaper, applied on read by BOTH the Actions v2 handler and the simulator. A preview computed by different code from the token it previews is a preview of nothing
- Strict JSON decoding (`decodeJSONStrict`) on all mutation endpoints
- Injectable dependency pattern for testability (`deps.go` files)
- Zitadel Actions v2 is the only source-of-truth claim integration path
- Backend is the single mutation authority — frontend and triggers signal, backend decides
- Internal contracts (FE->BE, BE->Sync) are self-defined but isolated from Zitadel-facing boundary
- Syndra-mediated Zitadel mutations always leave a trace before the Management API call (intent ledger for direct grants, outbox for bundle/rule cascades) — a Zitadel-side change with no such trace is never silently trusted, it is detected as drift and triaged (Wave 2 · Part 4)

## Mandatory Workflow

**Before any work (every prompt):**
- Consult OpenSpec (`openspec/INDEX.md`, relevant specs/changes) for intent, contracts, and status
- Use `codebase-memory-mcp` (`search_graph`, `trace_path`, `get_code_snippet`, `query_graph`, `get_architecture`, `search_code`) for code discovery *before* falling back to Grep/Glob/Read
- **Token-frugal navigation (always):** prefer targeted tools over broad reads — `codebase-memory-mcp` for structure/callers/impact, LSP (go-to-def, references, symbols, hover) for precise symbol navigation, and Grep/Read with narrow `offset`/`limit` only when those don't fit. Never read whole files or sweep directories when a targeted query answers it. Delegate bulk/multi-file reading to subagents (their output stays out of the main context) and hand off large artifacts as files rather than pasting them into prompts.

**After any code change (always, unprompted), scaled to the change:**
1. **OpenSpec alignment** — verify or create the relevant change under `openspec/changes/`, update `proposal.md`, `tasks.md`, `design.md`, and affected `specs/*.md` so the documented intent matches the code. If the change is already scoped, tick tasks and keep deltas coherent. Flag any drift between specs and reality.
2. **Meaningful tests** — add/extend tests that exercise the new behavior and its failure modes (not just happy-path coverage padding). Run `go test ./... && go vet ./...` in affected modules (`backend/`, `sync/`) and `bun run test && bun run lint` in `ui/`.
3. **Codebase memory refresh** — run `mcp__codebase-memory-mcp__detect_changes` and re-index affected scope so the graph reflects new/renamed/deleted symbols, routes, and call edges. Update any stale ADRs via `manage_adr` if architectural decisions shifted.

Scale depth to the task: a one-line typo fix does not need a full spec revision, but any behavioral or contract change does. When in doubt, do more, not less.

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
