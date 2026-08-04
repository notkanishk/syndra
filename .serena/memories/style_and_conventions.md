# Code Style & Conventions

## Go Backend
- **stdlib only**: No frameworks, no gRPC. Uses `net/http` for routing (Go 1.22+ method patterns).
- **`any` over `interface{}`**: Go 1.18+ idiom.
- **Injectable deps**: Each package has a `deps.go` with module-level function vars for testability. Tests use `resetDeps(t)` with `t.Cleanup`.
- **Strict JSON decoding**: `DisallowUnknownFields` + trailing token check via `contracts.go`.
- **Error handling**: Log and continue for non-critical failures (cache, audit). Propagate for critical failures (DB, auth).
- **Logging**: `log.Printf` with prefixed tags like `[ZITADEL]`, `[CACHE WARN]`, `[ERROR]`.
- **No emoji in log output** (cleaned up in audit).
- **Comments**: Only where logic isn't self-evident. No redundant docstrings.

## Frontend (TypeScript/React)
- **Bun runtime** for package management and dev server.
- **Generic typed fetchers**: `fetchServerJson<T>`, `fetchWithAuth<T>` with concrete return types.
- **Shared types** in `ui/src/lib/types.ts` mirroring Go models.
- **No `eslint-disable` comments** for `@typescript-eslint/no-explicit-any`.

## Testing
- Go: stdlib `testing` package, injectable deps, `t.Helper()`, `t.Cleanup()`.
- Frontend: Vitest with `@` path alias.
- Test file naming: `*_test.go` (Go), `__tests__/*.test.ts` (frontend).

## OpenSpec Workflow
- Changes documented in `openspec/changes/<change-name>/` with `proposal.md`, `design.md`, `tasks.md`.
- Feature coverage tracked in `openspec/changes/syndra-core-architecture/specs/feature-coverage.md`.
- Roadmap in `openspec/changes/syndra-core-architecture/ROADMAP.md`.
