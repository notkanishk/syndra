## Why

MkAuth had no local role storage, no role creation workflow, and no global role catalog. Roles existed only as opaque `(projectID, roleKey)` references scattered across `bundle_roles`, `mapping_rules`, and `direct_role_grants`. Creating a new role required manual Zitadel console interaction. There was no way to clone role metadata for accelerated setup, and no consolidated inventory for auditing unused or orphaned roles across projects.

## What Changes

* Adds a `roles` table (migration 000008) for locally-managed role metadata with `UNIQUE(zitadel_project_id, role_key)` constraint and clone provenance columns.
* Extends `ZitadelClient` with `AddProjectRole`, `ListProjectRoles`, and `UpdateProjectRole` methods for Zitadel Management API v1 role endpoints.
* Adds `POST /api/v1/roles` endpoint for role creation with optional `clone_from` parameter. When cloning, source role metadata (display name, description) is resolved from local DB or demo catalog and pre-populated into the new role. The role is propagated to Zitadel and persisted locally with provenance tracking.
* Adds `GET /api/v1/roles` endpoint returning a global role catalog — a computed merge of three sources (local MkAuth roles, demo catalog, DB-referenced roles) with per-role usage counts (bundles, mapping rules, assigned users) and unused flagging.
* Global disambiguation via `DisplayLabel` field (`"ProjectName: DisplayName"` format).

## Capabilities

### New Capabilities
* `role-creation`: Create roles in Zitadel projects via MkAuth with local metadata persistence.
* `role-cloning`: Snapshot & Fork — clone existing role metadata when creating new roles.
* `global-role-catalog`: Consolidated role inventory with usage metrics and unused detection.
* `role-disambiguation`: Project-prefixed display labels in global views.

### Modified Capabilities
* `role-management`: Not integrated -> Integrated in feature coverage.

## Impact

* Adds `roles` table (migration 000008).
* Modifies `orchestrator.go` (3 new interface methods), `client.go` (3 new implementations), `repositories.go` (6 new functions), `models.go` (2 new types), `deps.go` files (injectable vars), `router.go` (2 new routes).
* Creates `roles.go` (handler), `services/roles.go` (service layer).
* 20 new tests (11 handler + 6 service + 3 Zitadel client).
* Zero new go.mod dependencies. Graceful degradation to local-policy-only mode when Zitadel credentials absent.
