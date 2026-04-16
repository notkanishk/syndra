# Provisioning Intents — Implementation Record

**Phase:** 4 | **Status:** Complete | **Tests:** 22

## What Was Built
Structured provisioning intent pipeline: backend emits intents on grant changes, sync service consumes them via atomic claim API.

### Capabilities
- `provisioning_intents` table with 4-state machine (pending/acknowledged/completed/failed)
- `FlattenLLDAPGroup` using stable project ID (not mutable display name)
- `EmitProvisioningIntent` with idempotency deduplication and audit logging
- Webhook integration: grant_added/changed emit "add" intents, grant_removed emits "remove"
- Sync service API: `POST /api/v1/intents/claim` (atomic, `FOR UPDATE SKIP LOCKED`), complete, fail

### Key Design Choices
- Stable project ID for LLDAP group names (project renames never orphan groups)
- Atomic claim replaces separate get+acknowledge (prevents double-pickup)
- Intent failure is non-fatal to preserve existing webhook processing guarantees

## Key Files
- `backend/internal/services/provisioning.go` — intent emission
- `backend/internal/services/lldap.go` — group flattening
- `backend/internal/handlers/intents.go` — sync service + operator endpoints
- `backend/db/migrations/000009_provisioning_intents.up.sql`

## Verification
```bash
cd backend && go test ./... && go vet ./...  # 170 tests
```
