## Rationale

The sync service is the runtime component of Syndra's Bridge Plane. It closes the loop between Zitadel identity events (captured as provisioning intents) and LLDAP infrastructure state (group memberships and passwords). The service is deliberately minimal: it has no business logic, no policy decisions, and no public interface — it is a pure executor.

Key design choices:
- **Separate Go module**: No shared code with the backend. The only contract is the REST API. This avoids pulling LDAP dependencies into the backend and keeps deployment units independent.
- **External LLDAP compatibility**: The sync service is responsible for reaching LLDAP over the network; LLDAP does not need to run inside the Syndra Docker Compose stack. A separately managed deployment, such as an LLDAP server running in its own Proxmox LXC, is a valid target.
- **Zitadel UID as LLDAP uid**: Stable, immutable identifier. `displayName` and `mail` are synced for human readability during manual LLDAP audits.
- **`member` attribute on group DN**: Standard LDAP group membership pattern. The group entry is modified, not the user entry.
- **Single LDAP connection + mutex**: LLDAP is a lightweight Rust-based server for small deployments (makerspaces). Connection pooling adds unnecessary complexity.
- **Per-UID locking**: Intents for the same user serialize naturally via a `UIDLocker` map. Different users proceed in full parallelism.
- **Interfaces for testability**: `BackendClient` and `LDAPPool` interfaces enable mock-based unit testing without live servers.

## Open Question / Research Blocker

The current sync-service design assumes that Syndra can fetch a pre-hashed shadow credential from the backend and apply it to LLDAP through the normal LDAP write path. That assumption is not yet validated against the real target deployment.

Before the LLDAP/password bridge is treated as production-ready, Syndra needs clarity on:
- whether LLDAP accepts the intended password update mechanism at all
- whether LLDAP accepts pre-hashed credentials in the format Syndra stores
- whether the community-managed Proxmox LXC deployment behaves the same way as upstream-documented LLDAP installs

Until that research is complete, end-to-end password sync to LLDAP is considered paused even though the surrounding sync-service code exists.

## Technical Specification

### 1. Configuration

Environment variables with defaults:
- `BACKEND_URL` (default `http://backend:8080`), `SYNDRA_API_KEY` (required)
- `LLDAP_URL` (default `ldaps://lldap:636` for local/containerized development; in production this often points to an external host such as a Proxmox LXC), `LLDAP_BIND_DN` (required), `LLDAP_BIND_PASSWORD` (required), `LLDAP_BASE_DN` (default `dc=example,dc=com`), `LLDAP_INSECURE_SKIP_VERIFY` (default `false`)
- `SYNC_POLL_INTERVAL` (default `10s`), `SYNC_WORKER_COUNT` (default `5`), `SYNC_INTENT_LIMIT` (default `50`)

### 2. Backend HTTP Client

Consumes the existing backend API (no backend changes for intent operations):
- `POST /api/v1/intents/claim?limit=N` — atomically claim pending intents
- `POST /api/v1/intents/{id}/complete` — mark intent completed
- `POST /api/v1/intents/{id}/fail` — mark intent failed
- `GET /api/v1/shadow-credentials/{uid}/hash` — get shadow password hash (404 = no credential, not an error)
- `GET /api/v1/users/{uid}/profile` — get display name + email (new endpoint)

All requests include `Authorization: Bearer <SYNDRA_API_KEY>`.

### 3. LLDAP Client

Single `*ldap.Conn` with auto-reconnect on connection errors. Operations:
- `EnsureUser(uid, displayName, email)` — search by uid, create with objectClass `inetOrgPerson` if absent, update displayName/mail if changed
- `EnsureGroup(lldapGroup)` — search by cn, create with objectClass `groupOfNames` if absent
- `AddUserToGroup(uid, group)` — ModifyRequest Add `member` on group DN. Idempotent (ignore code 68).
- `RemoveUserFromGroup(uid, group)` — ModifyRequest Delete `member` on group DN. Idempotent (ignore code 16).
- `SetUserPassword(uid, hash)` — currently modeled as a ModifyRequest replacing `userPassword` with a pre-hashed Argon2id value, but this behavior is now considered provisional pending research against the real LLDAP target.

DN patterns:
- User: `uid=<targetUID>,ou=people,<baseDN>`
- Group: `cn=<lldapGroup>,ou=groups,<baseDN>`

### 4. Per-UID Ordering

`UIDLocker` provides per-UID mutual exclusion. Workers acquire the lock for a TargetUID before processing, release after. Map entries are cleaned up when no waiters remain (prevents unbounded growth).

### 5. Worker Pool

Polling loop claims intents on a ticker, dispatches to a buffered channel. Worker goroutines consume from the channel.

`processIntent` flow:
1. Acquire per-UID lock
2. For `action=add`: fetch user profile → `EnsureUser` → `EnsureGroup` → `AddUserToGroup`
3. For `action=remove`: `RemoveUserFromGroup`
4. Shadow password sync (best-effort, non-blocking on failure)
5. `CompleteIntent` or `FailIntent`
6. Release lock

Retry: exponential backoff (1s, 2s, 4s) for transient LDAP errors (ServerDown, network, Busy). Permanent errors (NoSuchObject, InsufficientAccess) fail immediately.

### 6. Backend User Profile Endpoint (new)

`GET /api/v1/users/{uid}/profile` with `withAPIKeyAuth`:
- Production: calls existing `zitadel.GetUser(uid)` → returns `{user_id, display_name, email}`
- Dev mode: looks up demo catalog → returns demo user profile
- 404 if not found

### 7. Docker Integration

Sync service added to `docker-compose.yml`:
- No `ports:` mapping (private worker)
- `depends_on: backend`
- Environment variables for LLDAP connection and polling config
- `restart: unless-stopped`

LLDAP itself is not required to be part of the same Compose deployment. The supported production shape is that Syndra runs its own containers while the sync service connects to an externally hosted LLDAP server over `LLDAP_URL`.

### 8. Graceful Shutdown

Signal handling via `signal.NotifyContext(SIGINT, SIGTERM)`. On signal: stop ticker, close intent channel, wait for workers to drain, close LDAP connection.

## Verification

```bash
cd sync && go build ./...
cd sync && go vet ./...
cd sync && go test ./...
cd backend && go build ./... && go test ./...
docker compose build sync
```
