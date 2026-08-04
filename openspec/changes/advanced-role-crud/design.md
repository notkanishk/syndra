## Rationale

The spec defines four capabilities: role creation with Zitadel propagation, snapshot & fork cloning, global role catalog with usage metrics, and project-prefixed disambiguation. The design stores only Syndra-created roles locally (not a full Zitadel sync) and builds the global catalog as a computed merge at query time.

## Technical Specification

### 1. Local Role Storage

`roles` table stores Syndra-managed role metadata. The `UNIQUE(zitadel_project_id, role_key)` constraint mirrors Zitadel's natural key. `cloned_from_project` and `cloned_from_role` are nullable provenance columns — informational only, no cascading relationship.

Roles created through other channels (Zitadel console, demo catalog) are NOT synced here. The global catalog merges sources at query time.

### 2. Zitadel API Integration

Three new methods on `ZitadelClient`:
- `AddProjectRole` → `POST /management/v1/projects/{projectId}/roles`
- `ListProjectRoles` → `POST /management/v1/projects/{projectId}/roles/_search`
- `UpdateProjectRole` → `PUT /management/v1/projects/{projectId}/roles/{roleKey}`

When `MgmtClient` is nil (local-policy-only mode), Zitadel propagation is skipped and the role is stored locally only.

### 3. Clone Flow (Snapshot & Fork)

`POST /api/v1/roles` with optional `clone_from: {project_id, role_key}`:

1. Resolve source role metadata: local DB first (must be `pgx.ErrNoRows` to fall through — real DB errors surface immediately), then demo catalog, else 404.
2. Pre-populate empty `display_name` and `description` from source. Explicit values take precedence.
3. Persist locally first — catches duplicates before touching Zitadel.
4. Propagate to Zitadel (if client available). If Zitadel fails, compensating rollback deletes the local row.
5. Audit log: `role.created`.

The local-first-then-Zitadel ordering ensures Syndra never tracks a role that doesn't exist upstream (P1 fix). The clone source lookup distinguishes "row not found" from actual DB errors to avoid masking backend faults as 404 (P2 fix).

The clone is a creation-time convenience. No ongoing sync between source and fork.

### 4. Global Role Catalog

`GET /api/v1/roles` merges three sources into a deduplicated map keyed by `(projectID, roleKey)`:

1. **Local roles** (source = "syndra") — highest priority for metadata.
2. **Demo catalog** (source = "demo") — from `demo.Projects()`.
3. **Referenced roles** (source = "referenced") — any `(projectID, roleKey)` pair in `bundle_roles`, `mapping_rules`, or `direct_role_grants` not already in sources 1 or 2.

Per-role computed fields:
- `BundleCount`: rows in `bundle_roles` referencing this role.
- `RuleCount`: rows in `mapping_rules` referencing this role as source or target.
- `AssignedUserCount`: distinct users in `direct_role_grants` (non-expired).
- `IsUnused`: `BundleCount + RuleCount == 0 && AssignedUserCount == 0`.
- ~~`DisplayLabel`: `"ProjectName: DisplayName"` (global disambiguation).~~ Removed by `ui-capability-gap-closure`; the UI composes the pair via `<RoleRef>` / `roleLabel()`.

Optional `?project_id=` query filter. Sorted by `(ProjectName, RoleKey)`.

### 5. Validation

- `role_key` must match `^[a-zA-Z0-9_-]+$` (same as Zitadel constraints).
- `project_id`, `role_key`, `display_name` all required (display_name can be empty if `clone_from` provided).
- `clone_from` requires both `project_id` and `role_key` when present.
- Duplicate `(project_id, role_key)` returns 409 CONFLICT.

### 6. Injectable Dependencies

All DB functions and service functions are injectable via `deps.go` for isolated testing.

## Verification

```bash
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./...  # 147 tests pass
```
