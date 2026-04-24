# Design: Live Zitadel Data Source

## Context

Auth boundary is live (PKCE OIDC, RS256 JWT validation); **data boundary is not**. The admin UI reads from hardcoded `demo.*` fixtures even with a fully configured Zitadel. This introduces a `directory` seam that swaps the source of truth based on whether `zitadel.MgmtClient` was successfully initialized at startup.

## Principle: least-churn flip

- Frontend: **no changes**. Same endpoints, same DTOs.
- Business logic: **minimal changes**. All `demo.*` direct calls become `directory.Default.*(ctx)`. Same return types.
- New package is narrow — only the directory-of-truth reads move through it. MkAuth-owned writes (bundles, mapping rules, direct grants, audit logs) stay authoritative in the local DB.

## Source interface

```go
// backend/internal/directory/directory.go
package directory

type Source interface {
    Users(ctx) ([]models.UserProfile, error)
    FindUser(ctx, userID string) (models.UserProfile, bool, error)
    Projects(ctx) ([]models.ProjectCatalog, error)
    FindProject(ctx, projectID string) (models.ProjectCatalog, bool, error)
    Applications(ctx) ([]models.ApplicationCatalog, error)
    FindApplication(ctx, appID string) (models.ApplicationCatalog, bool, error)
    RoleKeysForProject(ctx, projectID string) ([]string, error)
    ProjectName(ctx, projectID string) (string, error)
    InvalidateAll()
    InvalidateProject(projectID string)
    InvalidateUsers()
}

var Default Source
func Init()  // called from main.go after zitadel.InitClient()
```

## Implementations

### demoSource
Thin delegation to `demo.Users()`, `demo.Projects()`, `demo.Applications()`, etc. `Invalidate*` are no-ops. Lives in `backend/internal/directory/demo.go`.

### zitadelSource
Wraps the existing `zitadel.ZitadelClient` interface (`backend/internal/zitadel/orchestrator.go:58-76`) for users/projects/roles, and the `claim_profiles` table for application overlay.

**Mapping**:

| MkAuth model | Source | Notes |
|---|---|---|
| `UserProfile.ID` | `ZitadelUser.ID` | |
| `UserProfile.Name` | `ZitadelUser.DisplayName` (fallback `Username`) | |
| `UserProfile.Email` | `ZitadelUser.Email` | |
| `UserProfile.Status` | `ZitadelUser.State` → lowercased, normalized (`active`, `inactive`, `initial`, `locked`, `deleted`) | `USER_STATE_ACTIVE` → `active` |
| `UserProfile.Avatar` | Derived: initials of `DisplayName` | Same logic as UI `nameToAvatar` |
| `UserProfile.Title` / `.Team` / `.Location` | `""` | Not modeled in Zitadel |
| `ProjectCatalog.ID` / `.Name` | `ZitadelProject.ID` / `.Name` | |
| `ProjectCatalog.Kind` | `"zitadel"` constant | |
| `ProjectCatalog.Description` | `""` | |
| `ProjectCatalog.Roles[]` | `ListProjectRoles` → map `Key`, `DisplayName`→`Label`, `Group`→`Description` | Per-project call; cached 30s |
| `ApplicationCatalog.ID` | `ZitadelProject.ID` | One Application per live project |
| `ApplicationCatalog.Name` | `ZitadelProject.Name` | |
| `ApplicationCatalog.ProjectID` | `ZitadelProject.ID` | Self-reference; matches existing UI pattern |
| `ApplicationCatalog.ClaimName` | `claim_profiles.claim_name` if exists, else `"roles"` | |
| `ApplicationCatalog.FormatType` | `claim_profiles.format_type` if exists, else `"array"` | |
| `ApplicationCatalog.Consumer` / `.Description` | `""` | Pending Phase 5 admin CRUD |

### Caching

30-second TTL on list queries only; lookups (`FindUser`, `FindProject`) hit the list cache transparently.

```go
type cacheEntry[T any] struct {
    val     T
    expires time.Time
}
```

Keys: `"users"`, `"projects"`, `"project_roles:<id>"`, `"applications"`, `"claim_profiles"`. Stored in a `sync.Map`. Reads first check expiry; writes are plain `Store`.

**Invalidation** — called from `/api/v1/zitadel/*` mutation handlers:

- `POST /projects/{id}/roles`, `PUT /projects/{id}/roles/{key}`, `DELETE /projects/{id}/roles/{key}` → `InvalidateProject(id)` (drops `project_roles:<id>`, `projects`, `applications`).
- `POST /users/{id}/grants`, `PUT ... grants/{grantId}`, `DELETE ... grants/{grantId}` → no directory invalidation needed (grants aren't in the directory cache).
- Bulk fallback: `InvalidateAll()` — used by any future write path that touches multiple projects.

### `zitadelSource` failure mode

When a Zitadel call fails (timeout, 5xx, auth), the method returns the error up to the caller. Unlike the data-plane action endpoint, these are admin views — an error is strictly better than a fallback to stale/demo data. The UI already renders API error payloads; operators see the real failure. No silent fallback.

## Seed gating

`seed/demo.go`:

```go
func demoEnabled() bool {
    override := os.Getenv("MKAUTH_SEED_DEMO")
    if override != "" {
        return isTruthy(override)
    }
    // Default: off when live Zitadel is detected, on otherwise.
    return !liveZitadelConfigured()
}

func liveZitadelConfigured() bool {
    return os.Getenv("ZITADEL_DOMAIN") != "" && os.Getenv("ZITADEL_MACHINE_KEY_PATH") != ""
}
```

Log the decision: `[SEED] Live Zitadel detected; skipping demo seed (set MKAUTH_SEED_DEMO=true to override).`

Operators can still force seed on in live mode (useful for local integration tests against a staging Zitadel) by setting `MKAUTH_SEED_DEMO=true` explicitly. The seed can still write bundles/rules/audit to the DB — those records reference Zitadel project IDs as strings, and if the operator's real Zitadel doesn't have a `"printing"` project, the seed rows become harmless dangling references (already handled by `Topology` via "placeholder nodes" behavior).

## Threading `ctx`

Two signature changes:

- `upsertRole(roleMap, key, isSource, reason)` → `upsertRole(ctx, roleMap, key, isSource, reason)`. Needs ctx to look up project names via directory.
- `allApplicationProjectIDs()` → `allApplicationProjectIDs(ctx)`. Called in 2 places in `handlers/access.go`.

## Testing strategy

Mirror `services/views_test.go:14-32`'s injectable-deps pattern:

- `backend/internal/directory/directory_test.go`:
  - `TestDemoSource_Passthrough` — demo source returns what `demo.Users()`/`demo.Projects()` return.
  - `TestZitadelSource_Users` — mock `ZitadelClient.ListUsers` → expected `UserProfile` shape (DisplayName → Name, initials → Avatar, state normalization).
  - `TestZitadelSource_Projects_ExpandsRoles` — mock returns 2 projects, each with 3 roles; verify per-project `ListProjectRoles` called once and cached.
  - `TestZitadelSource_Applications_OverlayMerge` — inject `listClaimProfiles` stub returning one row; verify that row's `claim_name`/`format_type` override the default, and other projects get defaults.
  - `TestZitadelSource_Cache_TTL` — two back-to-back `Users(ctx)` → one upstream call; advance fake clock past 30s → second call reaches upstream.
  - `TestZitadelSource_Cache_InvalidateProject` — after `InvalidateProject("p1")`, next `Projects(ctx)` re-fetches; `project_roles:p2` cache untouched.
  - `TestZitadelSource_UpstreamError_Surfaces` — `ListUsers` returns error; `directorysource.Users(ctx)` returns the same error (no fallback).
  - `TestInit_DemoMode` — `zitadel.MgmtClient == nil` → `Default` is `demoSource`.
  - `TestInit_LiveMode` — `zitadel.MgmtClient = mockClient` → `Default` is `zitadelSource`.

Service tests in `views_test.go` and `roles_test.go` continue to work because `directory.Default` defaults to `demoSource` when `zitadel.MgmtClient` is nil (the default in tests).

## Non-goals

- Not changing the frontend. No new React components, no Zitadel API calls from the browser. The proxy pattern stays as-is.
- Not adding an Applications admin CRUD. `ApplicationCatalog.Consumer`/`.Description` stay empty in live mode — that's a Phase 5 deferred item.
- Not adding a search parameter to `ListUsers`/`ListProjects`. Pagination caps at `DefaultSearchLimit = 500` (same as the `/zitadel` diagnostics page), matching makerspace scale.
- Not introducing a background refresher. 30s TTL + on-write invalidation is sufficient.

## Migration path (for the operator)

1. Deploy the new backend.
2. If `ZITADEL_DOMAIN` + `ZITADEL_MACHINE_KEY_PATH` are set: on next restart, the `[DIRECTORY] Source=zitadel` log appears; UI immediately shows the operator's real directory.
3. If the operator previously ran with demo seed and wants a clean state: `DELETE FROM user_bundle_assignments` / `direct_role_grants` / `bundles` (or leave them — they stay as MkAuth local policy on top of the live directory).
4. Reverting is a single env-var unset (`unset ZITADEL_MACHINE_KEY_PATH`) — the next restart goes back to demo source.
