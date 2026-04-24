## Why

MkAuth's admin UI still shows **demo catalog data** (Alice Rivera, Sam Patel, the Laser Lab) even when `ZITADEL_DOMAIN`, `ZITADEL_MACHINE_KEY_PATH`, Actions v2 signing key, and webhook listener are all correctly configured. The zero-trust auth boundary is complete end-to-end (OIDC login, RS256 JWT validation, token forwarding), but the **data boundary** is not — `services/views.go` and `services/roles.go` still call `demo.Users()`, `demo.Projects()`, `demo.Applications()` directly, with no live-mode branch.

Concretely, every admin view (`/users`, `/projects`, `/bundles`, `/policies`, `/applications`, `/graph`, audit) reads from hardcoded fixtures in `backend/internal/demo/catalog.go` instead of from the already-live Zitadel Management API client (`zitadel.MgmtClient`). The only Zitadel-native page today is `/zitadel` (diagnostics), which is siloed from the normal admin navigation.

This change closes that gap so that a Zitadel-configured deployment displays the operator's real directory (users, projects, roles) without any further code change, while preserving the demo catalog as a local-dev fallback when `ZITADEL_DOMAIN` is unset.

## What Changes

- Adds a new `backend/internal/directory/` package with a `Source` interface and two implementations: `demoSource` (wraps the existing `demo.Catalog` package) and `zitadelSource` (wraps `zitadel.ZitadelClient` + `claim_profiles` DB table).
- `directory.Default` is initialized once at startup in `cmd/api/main.go` — picks `zitadelSource` when `zitadel.MgmtClient != nil`, else `demoSource`. Startup log indicates which source is active.
- Migrates 12 call sites off `demo.*` direct calls:
  - `services/views.go` — `Catalog`, `ListUsers`, `ListProjects`, `ListApplications`, `SimulateApplication`, `BundleImpact`, `Topology`, `ExplainUserAccess`, `upsertRole` (threaded `ctx`).
  - `services/roles.go` — `GlobalRoleCatalog`, `resolveRoleMetadata`.
  - `handlers/access.go` — `allApplicationProjectIDs` (takes `ctx`; 2 in-file callers updated).
- Adds a simple 30s TTL cache on Zitadel list queries (`ListUsers`, `ListProjects`, per-project `ListProjectRoles`) to avoid N+1 Zitadel calls on view-heavy page loads. Cache is invalidated from the `/api/v1/zitadel/*` mutation handlers after successful writes.
- `seed/demo.go` — `EnsureDemoData` short-circuits user/project seeding when live Zitadel is detected. DB-owned seed items (bundles, mapping rules, audit examples, access requests) still seed only when `MKAUTH_SEED_DEMO=true` is set; the default becomes *off in live mode, on in demo mode*.
- Applications in live mode: one `ApplicationCatalog` is synthesized per live Zitadel project, overlaid with any existing `claim_profiles` row for that project. Name comes from the Zitadel project name; defaults are `claim_name="roles"` and `format_type="array"` when no overlay exists. No new migration — `claim_profiles` already has `zitadel_project_id`, `claim_name`, `format_type`.

## Capabilities

### Modified Capabilities

- `user-management`: add requirement "User list is sourced from live Zitadel when configured" with scenario "ZITADEL_DOMAIN is set → GET /api/v1/users returns users from the Zitadel directory, not from the demo fixture".
- `service-catalog`: add requirement "Project and role pickers reflect live Zitadel projects and roles when configured".
- `application-claims`: note that application metadata (`ClaimName`, `FormatType`) is overlaid from `claim_profiles` on top of live Zitadel project discovery when a client is initialized.
- `demo-catalog`: status becomes "Local-dev only — bypassed when ZITADEL_DOMAIN is set".

## Impact

- **No migration required.** `claim_profiles` schema covers the overlay need.
- **Frontend contracts unchanged** — all UI pages continue to call the same `/api/proxy/catalog`, `/api/proxy/users`, `/api/proxy/projects`, `/api/proxy/applications` endpoints and receive the same DTO shapes.
- **Sync service unaffected.** `/api/v1/users/{uid}/profile` already uses `zitadel.MgmtClient` directly (`handlers/profile.go`), and sync only consumes `display_name` + `email` per `sync/internal/worker/worker.go:142-151`.
- **Creates**: `backend/internal/directory/{directory.go, demo.go, zitadel.go, directory_test.go}`.
- **Modifies**: `backend/internal/services/views.go`, `backend/internal/services/roles.go`, `backend/internal/handlers/access.go`, `backend/internal/handlers/discovery.go` (adds `directory.Invalidate*` calls after Zitadel mutations), `backend/internal/seed/demo.go`, `backend/cmd/api/main.go`, `.env.example`.
- **Living docs updated**: `openspec/INDEX.md`, `openspec/changes/mkauth-core-architecture/ROADMAP.md` (Phase 5 bullet), `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md`.
- **Known limitation**: `UserProfile.Title`, `.Team`, `.Location` are absent from Zitadel — they render as empty strings in live mode. These were presentational demo embellishments with no behavioral dependency. Sync only reads `DisplayName` + `Email`, so no breakage.
- **Known limitation**: `ApplicationCatalog.Description` and `.Consumer` are empty in live mode until an admin CRUD surface for application metadata exists (deferred Phase 5 "Service Catalog Abstraction").
