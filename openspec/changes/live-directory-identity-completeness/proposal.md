## Why

The `live-zitadel-data-source` change flipped the directory seam from demo fixtures to the live Zitadel Management API when `ZITADEL_DOMAIN` + machine key are configured. Live data now drives the admin UI — but an operator opening the Identity-section pages sees **incomplete** live data:

- **`/applications` page renders Zitadel *projects*, not *applications*.** The first-cut `zitadelSource.Applications()` synthesized one `ApplicationCatalog` per Zitadel project because the Management client had no `ListApplications` method. Real Zitadel applications — OIDC clients, APIs, SAML SPs attached to a project — were never fetched. Operators with multiple apps per project saw only the project; operators with apps-only-no-claim-profile saw no obvious type signal.
- **`/users` page always shows empty Title, Team** in live mode. The first-cut `toUserProfile()` hardcoded those fields to `""` because Zitadel's first-class user schema doesn't model them. The documented Zitadel mechanism for admin-managed per-user attributes is the user metadata K/V store (`POST /management/v1/users/{id}/metadata/_search`) — which Syndra didn't call.

This change closes both gaps so the Identity section is genuinely identity-complete in live mode.

## What Changes

- **Zitadel client surface grows two methods:**
  - `ListApplications(ctx, projectID, SearchParams) (*SearchResult[ZitadelApplication], error)` → `POST /management/v1/projects/{id}/apps/_search`. Derives `Type` (`OIDC` / `API` / `SAML`) from which `*Config` block the response carries. New `HumanizeAppType` helper exported so the directory layer can label apps for the UI's existing `consumer` column without a DTO/schema change.
  - `ListUserMetadata(ctx, userID, SearchParams) (*SearchResult[UserMetadata], error)` → `POST /management/v1/users/{id}/metadata/_search`. Base64-decodes metadata values so callers get plaintext.

- **`zitadelSource.Applications()` rewritten** to:
  - Fetch real applications via a bounded-parallel `errgroup` (concurrency 4) per live Zitadel project.
  - Tolerate per-project `ListApplications` failures (logged, skipped) rather than failing the whole page when one project's apps endpoint is down.
  - Preserve the existing `claim_profiles`-keyed-by-project overlay: all apps under project P inherit P's claim profile. This matches Zitadel Actions v2's grant-level (project-scoped) claim emission model.
  - Reuse the existing `consumer` field on `ApplicationCatalog` for the human-readable app type label (`OIDC Client` / `API` / `SAML SP`). **Zero UI contract change.**

- **`zitadelSource.Users()` enrichment**:
  - After `ListUsers`, fans out `ListUserMetadata` per user (errgroup limit 8, same 30s TTL cache).
  - Merges well-known keys (case-insensitive) into the existing `UserProfile` struct: `title` → `Title`, `team` → `Team`. Unknown keys ignored.
  - Per-user metadata fetch failures are logged and the user returns with empty Title/Team, never fails the whole list.
  - `FindUser` applies the same overlay on both the cached-list hit and the `GetUser` fallback.

- **Invalidation extended**: `InvalidateProject(projectID)` additionally drops the per-project apps cache; `InvalidateUsers()` also drops every user's metadata cache entry so a newly-added `title` metadata row shows up on the next page load, not only after the TTL.

## Capabilities

### Modified Capabilities

- `user-management`: `UserProfile.Title`, `.Team` are populated from Zitadel user metadata when the admin has set the corresponding keys; blank otherwise.
- `application-claims` / `service-catalog`: `/applications` renders **real Zitadel applications**, not project stubs. Type (`OIDC` / `API` / `SAML`) surfaces via the existing `consumer` column.

## Impact

- **No new migrations.** `claim_profiles` schema unchanged — still project-keyed, still overlays cleanly.
- **Frontend contracts unchanged.** All three Identity pages continue to consume the same DTOs (`UserListItem`, `ProjectSummary`, `ApplicationView`). `consumer` now conveys app type instead of being always-empty in live mode.
- **Creates:** `backend/internal/zitadel/{applications.go,applications_test.go,user_metadata.go}`, `openspec/changes/live-directory-identity-completeness/{proposal.md,design.md,tasks.md}`.
- **Modifies:** `backend/internal/zitadel/orchestrator.go` (interface), `backend/internal/directory/{zitadel.go,directory_test.go}`, `backend/internal/seed/demo_test.go`, `backend/internal/services/roles_test.go`, `backend/internal/zitadel/orchestrator_test.go` (mock extensions), `openspec/INDEX.md`, `openspec/changes/syndra-core-architecture/ROADMAP.md`, `openspec/changes/syndra-core-architecture/specs/feature-coverage.md`.
- **Known gap (intrinsic):** `ProjectCatalog.Description` remains empty in live mode. Zitadel's v1 `ListProjects` response carries no description field. Not in scope for this change.
