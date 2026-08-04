## 1. Directory package

- [x] 1.1 Create `backend/internal/directory/directory.go` — `Source` interface, package `Default Source` var, `Init()` selector, `Tag()`/`Invalidate*` methods on the interface.
- [x] 1.2 Create `backend/internal/directory/demo.go` — `demoSource` implementing `Source` via existing `demo.*` helpers.
- [x] 1.3 Create `backend/internal/directory/zitadel.go` — `zitadelSource` wrapping `zitadel.ZitadelClient` + `claim_profiles` reader. 30s TTL cache via `sync.Map`.
- [x] 1.4 Add `db.ListClaimProfiles(ctx) ([]ClaimProfileRow, error)` to read `zitadel_project_id, claim_name, format_type` from the `claim_profiles` table.

## 2. Migrate services

- [x] 2.1 `services/views.go` — `Catalog(ctx)` → `directory.Default.Users/Projects/Applications(ctx)`; propagates error. Handler signature updated (`handlers/views.go:handleGetCatalog`).
- [x] 2.2 `services/views.go` — `ListUsers`, `ListProjects`, `ListApplications`, `SimulateApplication`, `BundleImpact`, `Topology`, `ExplainUserAccess` → directory.
- [x] 2.3 `services/views.go` — `upsertRole(ctx, ...)` signature gains leading `ctx`; project name lookup via `directory.Default.ProjectName(ctx, id)`. Callers in `collectUserRoles` updated, plus `views_test.go:TestUpsertRoleDeduplicatesReason`.
- [x] 2.4 `services/roles.go` — `GlobalRoleCatalog`: demo-catalog branch now reads from `directory.Default.Projects(ctx)`; the source label matches `directory.Default.Tag()` (`demo` or `zitadel`). Project-name resolution pre-indexed from the same `dirProjects` slice.
- [x] 2.5 `services/roles.go` — `resolveRoleMetadata`: demo fallback replaced with `directory.Default.FindProject(ctx, id)`.

## 3. Migrate handlers

- [x] 3.1 `handlers/access.go` — `allApplicationProjectIDs(ctx)` → calls `directory.Default.Applications(ctx)`. Updated 2 callers (`handleUpsertUserDirectGrant`, `handleResolveAccessRequest`). On directory error, logs and returns `nil` so the cache rebuild fails soft.
- [x] 3.2 `handlers/discovery.go` — after successful `handleCreateZitadelProjectRole`, `handleUpdateZitadelProjectRole`, `handleDeleteZitadelProjectRole`, call `directory.Default.InvalidateProject(id)`.
- [x] 3.3 `handlers/profile.go` — left untouched. Already correctly branches on `zitadel.MgmtClient`; adding a directory-layer hop would churn tests without clarifying intent.

## 4. Seed + main wiring

- [x] 4.1 `seed/demo.go` — `demoEnabled()` defaults to `false` when `ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH` are set; `SYNDRA_SEED_DEMO` env var is the explicit override (both directions). Log explains the decision.
- [x] 4.2 `cmd/api/main.go` — reordered: `zitadel.InitClient()` runs before the seed, then `directory.Init()` selects the source and emits `[DIRECTORY] Source=...`, then seed runs (and skips itself in live mode).
- [x] 4.3 `.env.example` — documents `SYNDRA_SEED_DEMO` behavior change and the live-mode directory switch.

## 5. Tests

- [x] 5.1 `TestDemoSource_Users_ReturnsSeeded`, `TestDemoSource_FindProject_PresentAndAbsent`, `TestDemoSource_Tag` — demo source parity with existing fixtures.
- [x] 5.2 `TestZitadelSource_Users_MapsFields` — DisplayName/Username fallback, state normalization (`USER_STATE_ACTIVE → active`, `_LOCKED → locked`), avatar initials.
- [x] 5.3 `TestZitadelSource_Projects_ExpandsAndCachesRoles` — per-project `ListProjectRoles` fans out correctly; second `Projects()` call hits cache (1 upstream call each).
- [x] 5.4 `TestZitadelSource_Applications_OverlayFromClaimProfiles` — `claim_profiles` row overrides default `roles/array`; other projects keep defaults.
- [x] 5.5 `TestZitadelSource_Cache_TTLExpires` — within TTL: 1 upstream call; past TTL: second call re-fetches.
- [x] 5.6 `TestZitadelSource_InvalidateProject_DropsOnlyTargeted` — `InvalidateProject("p1")` drops `project_roles:p1`, `projects`, and `applications`; next `Projects()` re-fetches.
- [x] 5.7 `TestZitadelSource_Users_UpstreamErrorSurfaces` — upstream error wrapped and surfaced; no silent fallback.
- [x] 5.8 `TestZitadelSource_FindUser_FallsBackToGetUser` — list-cache miss triggers direct `GetUser` call.
- [ ] 5.9 Seed-mode selection tests deferred — covered indirectly by `directory.Default.Tag()` behavior and env-var smoke.

## 6. OpenSpec deltas

- [x] 6.1 Created `openspec/changes/live-zitadel-data-source/` with `proposal.md`, `design.md`, `tasks.md`.
- [x] 6.2 Updated `openspec/INDEX.md` — added Change Log row, bumped `user-management` / `service-catalog` / `demo-catalog` capability status to note the live-Zitadel source, added `live-zitadel-data-source` to Phase 5 column.
- [x] 6.3 Updated `openspec/changes/syndra-core-architecture/ROADMAP.md` — added completed Phase 5 bullet.
- [x] 6.4 Updated `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` — `demo-catalog`, `user-management`, `service-catalog` rows reflect directory-based source; implementing-changes map extended.

## 7. Verification

- [x] 7.1 `go vet ./... && go build ./...` in `backend/` — clean.
- [x] 7.2 `go test ./...` in `backend/` — 253 tests pass (10 new in `internal/directory/`; existing suite unchanged because tests run in demo mode, which is the default when `zitadel.MgmtClient` is nil).
- [x] 7.3 Frontend: `bun run test && bun run lint` in `ui/` — 6 tests pass, no lint errors.
- [ ] 7.4 Manual smoke on the operator's configured Zitadel: confirm `[DIRECTORY] Source=zitadel` log line at startup; load `/users`, `/projects`, `/bundles`, `/applications` and verify real Zitadel entities appear instead of Alice/Sam/Laser-Lab.
- [ ] 7.5 Demo mode regression: unset `ZITADEL_MACHINE_KEY_PATH`, restart → `[DIRECTORY] Source=demo` log, UI unchanged.
- [ ] 7.6 `mcp__codebase-memory-mcp__detect_changes` + reindex the new `internal/directory/` package.
