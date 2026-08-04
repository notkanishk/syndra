# Design: Live-Directory Identity Completeness

## Context

After `live-zitadel-data-source` landed, the `directory.Source` seam reads live Zitadel state — but the first cut had two known-incomplete areas that read as "data missing" to operators:

1. Applications were fabricated from projects because the Management client didn't implement `ListApplications`.
2. User Title/Team were hardcoded empty because Zitadel's user schema doesn't model them as first-class fields.

This change completes both areas without touching the frontend.

## Principle: no contract change, no new schema

- `ApplicationCatalog` DTO stays as-is. The existing `consumer` field is repurposed from "always empty in live mode" to "human-readable app type label" (`OIDC Client` / `API` / `SAML SP`). The UI already renders this column.
- `UserProfile.Title/Team` already exist. We just start populating them from the Zitadel user metadata K/V store when the admin has set the corresponding keys.
- No database migration. `claim_profiles` stays project-keyed.

## Zitadel client extensions

```go
// backend/internal/zitadel/applications.go
type ZitadelApplication struct {
    ID    string
    Name  string
    State string
    Type  string // "OIDC" | "API" | "SAML" | ""
}
func HumanizeAppType(t string) string // "OIDC Client" | "API" | "SAML SP" | ""

func (c *managementClient) ListApplications(ctx, projectID, SearchParams) (*SearchResult[ZitadelApplication], error)
```

Zitadel v1 does not emit a top-level app-type discriminator. The response shape is:
```json
{"details": {...}, "result": [{"id": "...", "name": "...", "state": "...", "oidcConfig"|"apiConfig"|"samlConfig": {...}}]}
```
`Type` is derived by checking which `*Config` block is non-empty. If none is, `Type == ""` and `HumanizeAppType` returns `""` — `consumer` stays blank for unrecognized app shapes (safer than guessing).

```go
// backend/internal/zitadel/user_metadata.go
type UserMetadata struct {
    Key   string
    Value string // base64-decoded at read time
}

func (c *managementClient) ListUserMetadata(ctx, userID, SearchParams) (*SearchResult[UserMetadata], error)
```

Zitadel stores metadata values as base64 on the wire to support arbitrary bytes. `decodeMetadataValue` falls back to the raw string if the value isn't valid base64 — this keeps test fixtures and any future protocol change from rendering blank.

## Directory-layer changes

### Applications: per-project fan-out with tolerance

```
for each project P in Projects(ctx):       // cached
    errgroup.Go(limit=4): fetchProjectApps(P.ID)  // cached per project
merge all apps, overlay claim_profiles keyed by P.ID
```

- **Concurrency limit = 4**: enough to mask per-project latency, low enough not to hammer Zitadel. Makerspace-scale (tens of projects) stays well under rate-limit thresholds.
- **Partial failure**: a single project's `ListApplications` error is logged and its apps are skipped. The rest of the page still renders. Rationale: admins shouldn't lose the whole list because one project is in a weird state — Zitadel has fine-grained error paths (missing permission on one project, etc.) that are specific, not global.
- **Partial results are NOT cached globally.** A partial catalog cached for 30s would (a) silently mislead `/applications` for the full TTL window and (b) — more importantly — let the missing project's compiled claims get erased: `cacheRebuildUser` deletes every `mapping:<user>:*` key before rebuilding, and any caller that derives project scope from `Applications()` would skip the failed project. We track partial state with an `atomic.Bool` during fan-out and skip the global cache write when set. Per-project caches still persist successful fetches independently. The downstream caller `cacheRebuildProjectIDs` was also re-pointed at `Projects()` (which has its own resilient cache) so the data plane never depends on `Applications()` completeness for the rebuild scope.
- **Claim-profile overlay stays project-keyed**: all apps under a project inherit that project's claim_name / format_type. This matches Zitadel Actions v2's grant-level (project-scoped) claim emission — the shaping rule is applied at grant time, not per-app.

### Users: metadata overlay

```
users := paginate(ListUsers)
errgroup.Go(limit=8): for each u in users: fetchUserMetadata(u.ID)  // cached per user
for each well-known key in the metadata: write into the corresponding UserProfile field
```

- **Concurrency limit = 8**: higher than apps because metadata endpoints are cheaper (keyed lookups, no nested joins) and the fan-out is per-user, potentially 10x wider.
- **Well-known key set**: `title`, `team`. Case-insensitive match — admins tripping over "Title" vs "title" would be a silent failure mode, so we normalize on read.
- **Per-user failure tolerance**: metadata fetch errors are logged, the user returns with the failed fields empty. Never blocks the rest of the user list.
- **Cache TTL = 30s** (same as list caches). Short enough that metadata edits in Zitadel console propagate quickly, long enough to absorb view-heavy page loads.

## Invalidation

- `InvalidateProject(projectID)` now drops `apps_by_project:<id>` in addition to `project_roles:<id>`, the global `projects` list, and `applications`. Follows the existing "drop anything that embeds project data" rule.
- `InvalidateUsers()` drops the `users` list and every `user_metadata:<id>` entry. Without this, a newly-set metadata row would only appear after the 30s TTL expired — jarring for an admin who just set it.
- `InvalidateApplications()` is not added as a separate Source method. No current handler mutates apps (Syndra doesn't surface app CRUD). Forcing an app-specific refresh goes through `InvalidateProject(projectID)`, which is the natural seam since apps belong to projects.

## Why not a new `type` field on ApplicationCatalog

Considered and rejected:
- Would require UI, TS type, and backend model changes.
- `consumer` is already in the DTO, already rendered, and already empty in live mode — there's a free slot for the semantic we want.
- Operators reading "OIDC Client" in the consumer column learn what the app is without needing a new column; the previous field was so under-specified ("consumer" never had operator-facing guidance) that overloading it here is a net clarity win.

## Error model summary

| Scenario | Behavior |
|---|---|
| `Projects()` fails | `Applications()` returns the same error (projects list is prerequisite). |
| Single project's `ListApplications` fails | Logged, project contributes 0 apps, other projects rendered. **Global cache write skipped** so next call retries the failed project; successful per-project fetches stay cached. |
| Single user's `ListUserMetadata` fails | Logged, user returns with empty Title/Team, list continues. |
| `ListUsers` fails | `Users()` returns the error (base list is required). |
| All apps pagination pages exhausted (>100) | `paginate` returns "pagination exceeded; refusing to spin" — same safety as existing Users/Projects paths. |
