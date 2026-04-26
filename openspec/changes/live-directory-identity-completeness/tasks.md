## 1. Zitadel client surface

- [x] 1.1 Create `backend/internal/zitadel/applications.go` — `ZitadelApplication` struct, `HumanizeAppType` helper, `managementClient.ListApplications` hitting `POST /management/v1/projects/{id}/apps/_search`. Derives `Type` from which `*Config` block is present.
- [x] 1.2 Create `backend/internal/zitadel/user_metadata.go` — `UserMetadata` struct, `managementClient.ListUserMetadata` hitting `POST /management/v1/users/{id}/metadata/_search`, base64-decoding values.
- [x] 1.3 Extend `ZitadelClient` interface in `orchestrator.go` with both new methods.

## 2. Directory layer

- [x] 2.1 `zitadelSource.Applications()` rewritten: `Projects(ctx)` + errgroup(SetLimit(4)) `ListApplications` per project. Claim-profile overlay keyed by project. Per-project failure tolerance. Cache global + per-project.
- [x] 2.2 `zitadelSource.Users()` enrichment: errgroup(SetLimit(8)) `ListUserMetadata` per user. Merge well-known keys (case-insensitive: `title`, `team`, `location`) into `UserProfile`. Per-user failure tolerance.
- [x] 2.3 `FindUser` applies same metadata overlay on `GetUser` fallback path.
- [x] 2.4 `InvalidateProject` drops `apps_by_project:<id>` in addition to existing keys.
- [x] 2.5 `InvalidateUsers` drops per-user metadata caches so newly-set metadata shows up without waiting for TTL.

## 3. Mock/stub extensions

- [x] 3.1 `backend/internal/seed/demo_test.go::stubClient` — no-op `ListApplications`, `ListUserMetadata`.
- [x] 3.2 `backend/internal/zitadel/orchestrator_test.go::mockClient` — no-op `ListApplications`, `ListUserMetadata`.
- [x] 3.3 `backend/internal/services/roles_test.go::failingRoleClient` — no-op `ListApplications`, `ListUserMetadata`.
- [x] 3.4 `backend/internal/directory/directory_test.go::mockDirClient` — injectable `listAppsFn` + `listMetadataFn`, mutex-guarded counter maps.

## 4. Tests

- [x] 4.1 `TestListApplications_ParsesOIDCAndAPITypes` — derived `Type` for OIDC / API / SAML response shapes.
- [x] 4.2 `TestListApplications_HitsCorrectEndpoint` — path is `/management/v1/projects/{id}/apps/_search`.
- [x] 4.3 `TestListUserMetadata_DecodesBase64Values` — base64 round-trip through client.
- [x] 4.4 `TestHumanizeAppType_KnownAndUnknown` — OIDC/API/SAML → labels; unknown → `""`.
- [x] 4.5 `TestZitadelSource_Applications_ReturnsRealApps` — 2 projects (1 + 2 apps) yield 3 distinct `ApplicationCatalog` entries with correct `ProjectID` and `Consumer` type label.
- [x] 4.6 `TestZitadelSource_Applications_OverlayAppliesPerProject` — claim profile on p1 flows to every app under p1; p2 falls back to defaults.
- [x] 4.7 `TestZitadelSource_Applications_EmptyProjectContributesNothing` — project with zero apps yields no entries.
- [x] 4.8 `TestZitadelSource_Applications_PartialFailureStillReturns` — one project's `ListApplications` error is tolerated; the other project's apps still render.
- [x] 4.9 `TestZitadelSource_Users_MetadataOverlay` — `title`/`Team` (mixed case) merge into `UserProfile`, unset keys stay empty.
- [x] 4.10 `TestZitadelSource_Users_MetadataErrorDoesntFailList` — a failing user doesn't block the rest of the list; healthy users still get overlay.
- [x] 4.11 `TestZitadelSource_Applications_PartialFailureNotCached` — partial fan-out result is NOT written to the global cache; once the broken project recovers, the next call sees the full catalog instead of serving the cached gap for 30s.
- [x] 4.12 `TestZitadelSource_Applications_FullSuccessCachesGlobally` — the all-success path still caches; second call hits cache (no extra upstream).

## 4b. Post-review fixes (review feedback P1)

- [x] 4b.1 `handlers/access.go::cacheRebuildProjectIDs` — derives scope from `directory.Default.Projects(ctx)` (was `Applications(ctx)`). Prevents partial Applications fan-out failures from silently erasing compiled claims for projects whose apps were transiently unreachable. Renamed from `allApplicationProjectIDs`.
- [x] 4b.2 `zitadelSource.Applications` — tracks partial fan-out via `atomic.Bool`; on partial failure, returns the partial result (admins still see what's available) but skips the global cache write so the next call retries the failed projects. Per-project caches still persist successful fetches.
- [x] 4b.3 gofmt all changed files (`directory/zitadel.go`, `zitadel/applications.go`, `zitadel/orchestrator_test.go`, `seed/demo_test.go`, `services/roles_test.go`, `handlers/access.go`).

## 5. OpenSpec deltas

- [x] 5.1 Created `openspec/changes/live-directory-identity-completeness/` with `proposal.md`, `design.md`, `tasks.md`.
- [x] 5.2 Updated `openspec/INDEX.md` — added Change Log row.
- [x] 5.3 Updated `openspec/changes/mkauth-core-architecture/ROADMAP.md` — Phase 5 bullet.
- [x] 5.4 Updated `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — `application-claims` and `user-management` rows reflect real-apps + metadata overlay.

## 6. Verification

- [x] 6.1 `go test ./... && go vet ./...` in `backend/` — all packages pass.
- [x] 6.2 `go test ./internal/directory/... -v` — 18 tests including the 6 new ones plus 2 added for partial-cache-skip; all pass.
- [x] 6.3 `go test ./internal/zitadel/... -v` — new applications_test.go parse cases pass.
- [ ] 6.4 Manual smoke on operator's live Zitadel: `/applications` shows real app names per project with `consumer` = `OIDC Client` / `API` / `SAML SP`. Setting a user's `title` metadata in Zitadel console causes the `/users` card to pick it up after ≤30s.
- [ ] 6.5 Graph refresh via `mcp__codebase-memory-mcp__detect_changes`.
