# MkAuth

Identity & Access Management orchestration layer for an academic makerspace. Companion to Zitadel with Google Workspace as sole IdP.

## Quick Navigation

- **Spec index:** `openspec/INDEX.md` — master hub for all specs, capabilities, and changes
- **Architecture:** `openspec/changes/mkauth-core-architecture/design.md` — three-plane design, Zitadel interaction matrix, IdP chain
- **Roadmap:** `openspec/changes/mkauth-core-architecture/ROADMAP.md` — phase timeline (1-4 complete, 5-6 ahead)
- **Reality check:** `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — planned vs integrated

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

- Strict JSON decoding (`decodeJSONStrict`) on all mutation endpoints
- Injectable dependency pattern for testability (`deps.go` files)
- Zitadel Actions v2 is the only source-of-truth claim integration path
- Backend is the single mutation authority — frontend and triggers signal, backend decides
- Internal contracts (FE->BE, BE->Sync) are self-defined but isolated from Zitadel-facing boundary

## Mandatory Workflow

**Before any work (every prompt):**
- Consult OpenSpec (`openspec/INDEX.md`, relevant specs/changes) for intent, contracts, and status
- Use `codebase-memory-mcp` (`search_graph`, `trace_path`, `get_code_snippet`, `query_graph`, `get_architecture`, `search_code`) for code discovery *before* falling back to Grep/Glob/Read

**After any code change (always, unprompted), scaled to the change:**
1. **OpenSpec alignment** — verify or create the relevant change under `openspec/changes/`, update `proposal.md`, `tasks.md`, `design.md`, and affected `specs/*.md` so the documented intent matches the code. If the change is already scoped, tick tasks and keep deltas coherent. Flag any drift between specs and reality.
2. **Meaningful tests** — add/extend tests that exercise the new behavior and its failure modes (not just happy-path coverage padding). Run `go test ./... && go vet ./...` in affected modules (`backend/`, `sync/`) and `bun run test && bun run lint` in `ui/`.
3. **Codebase memory refresh** — run `mcp__codebase-memory-mcp__detect_changes` and re-index affected scope so the graph reflects new/renamed/deleted symbols, routes, and call edges. Update any stale ADRs via `manage_adr` if architectural decisions shifted.

Scale depth to the task: a one-line typo fix does not need a full spec revision, but any behavioral or contract change does. When in doubt, do more, not less.
