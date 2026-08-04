# Sync Service — Implementation Record

**Phase:** 4 | **Status:** Complete | **Tests:** 32 (sync) + backend profile endpoint

## What Was Built
Independent Go binary (separate Docker container) polling backend intents and executing LLDAP mutations via `go-ldap/v3`.

### Capabilities
- Backend HTTP client consuming intent claim/complete/fail + shadow credential + user profile APIs
- LLDAP client: EnsureUser, EnsureGroup, AddUserToGroup, RemoveUserFromGroup, SetUserPassword
- Per-UID ordering via `UIDLocker` (same-user intents serialize, different users parallelize)
- Configurable worker pool (default 5) with exponential backoff on transient LDAP errors
- Graceful shutdown with signal handling and worker drain
- Backend `GET /api/v1/users/{uid}/profile` endpoint for display name + email sync

### Open Research
LLDAP password propagation (`SetUserPassword` with pre-hashed Argon2id) is unverified against real LLDAP. Group membership mutations are expected to work; password sync is paused.

### Configuration
- `BACKEND_URL`, `SYNDRA_API_KEY` — backend connection
- `LLDAP_URL`, `LLDAP_BIND_DN`, `LLDAP_BIND_PASSWORD`, `LLDAP_BASE_DN` — LLDAP connection
- `SYNC_POLL_INTERVAL` (10s), `SYNC_WORKER_COUNT` (5), `SYNC_INTENT_LIMIT` (50)

## Key Files
- `sync/cmd/sync/main.go` — entry point
- `sync/internal/worker/worker.go` — polling loop + worker pool
- `sync/internal/worker/ordering.go` — per-UID mutual exclusion
- `sync/internal/ldap/client.go` — LLDAP operations
- `sync/internal/backend/client.go` — backend HTTP client

## Verification
```bash
cd sync && go test ./... && go vet ./...
cd backend && go test ./... && go vet ./...
docker compose build sync
```
