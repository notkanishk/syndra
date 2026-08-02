# Advanced Role CRUD — Implementation Record

**Phase:** 3 | **Status:** Complete | **Tests:** 20

## What Was Built
Local role storage, Zitadel-propagated role creation with Snapshot & Fork cloning, and a global role catalog with usage metrics.

### Capabilities
- `POST /api/v1/roles` — create role with optional `clone_from`, Zitadel propagation, compensating rollback on failure
- `GET /api/v1/roles` — global catalog merging 3 sources (local, demo, referenced) with usage counts and unused flagging
- ~~`DisplayLabel` format: `"ProjectName: DisplayName"` for global disambiguation~~ — **superseded** by `ui-capability-gap-closure`: the field is removed and the composition moved to the UI, where it applies to every role reference rather than only catalog rows.
- Graceful degradation to local-only when Zitadel credentials absent

### Key Design Choices
- Local-first-then-Zitadel ordering with compensating delete on upstream failure
- Clone source resolved from local DB first, demo catalog fallback, `pgx.ErrNoRows` distinguished from real DB errors
- Roles table uses `UNIQUE(zitadel_project_id, role_key)` mirroring Zitadel's natural key

## Key Files
- `backend/internal/handlers/roles.go` — HTTP handlers
- `backend/internal/services/roles.go` — service layer with clone resolution
- `backend/db/migrations/000008_roles.up.sql`
- `backend/internal/zitadel/client.go` — AddProjectRole, ListProjectRoles, UpdateProjectRole

## Verification
```bash
cd backend && go test ./... && go vet ./...  # 147 tests
```
