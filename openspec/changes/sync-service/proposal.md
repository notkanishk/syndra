## Why

MkAuth can emit provisioning intents and store shadow credentials, but nothing consumes those intents to execute actual LLDAP mutations. Users who gain or lose roles in Zitadel see no change in their LLDAP group memberships or Samba access. The Bridge Plane described in the architecture has no runtime implementation.

## What Changes

* Adds a `sync/` directory containing an independent Go binary deployed as a separate Docker container alongside the existing backend, UI, Postgres, and Redis services, while connecting to LLDAP over the network.
* The sync service polls `POST /api/v1/intents/claim` on a configurable interval, dispatches claimed intents to a configurable worker pool, executes LLDAP mutations via `go-ldap/v3`, and reports results via the complete/fail endpoints.
* LLDAP user provisioning: on first "add" intent for a user, the service creates the LLDAP user entry with Zitadel UID as the canonical `uid`, plus `displayName` and `mail` attributes synced from the user's Zitadel profile for human readability.
* Shadow password sync: when processing any intent for a user, the worker checks `GET /api/v1/shadow-credentials/{uid}/hash` and is intended to sync password state to LLDAP if present, but the exact compatibility of that flow with real LLDAP deployments is now an explicit research question.
* Per-UID ordering ensures intents for the same user are processed sequentially while different users proceed in parallel.
* Adds a lightweight backend endpoint `GET /api/v1/users/{uid}/profile` (API-key auth) for the sync service to fetch user display names and emails.

## Capabilities

### New Capabilities

* `lldap-sync-service`: Concurrent provisioning worker executing LLDAP mutations from backend intents.
* `lldap-group-membership`: Add/remove users from LLDAP groups via LDAP ModifyRequest on group `member` attribute.
* `lldap-user-provisioning`: Auto-create LLDAP user entries with Zitadel UID + displayName + mail on first group assignment.
* `shadow-password-ldap-sync`: Sync Argon2id shadow credentials to LLDAP `userPassword` attribute.
* `user-profile-api`: Backend endpoint exposing user display name and email for internal service consumers.

### Modified Capabilities

* `provisioning-intents`: Intents are now consumed end-to-end (not just emitted and queued).

## Research Status

This change is implemented in code, but one part of the design remains unresolved: whether MkAuth's current password propagation model is actually compatible with the target LLDAP deployment, especially the externally hosted Proxmox LXC installation.

Because of that uncertainty, the LLDAP password-sync portion of this capability is paused pending research and real-environment validation. Other MkAuth work can continue independently.

## Impact

* Creates `sync/` directory (separate Go module, ~12 source files + tests).
* Adds sync service to `docker-compose.yml` (no public ports). LLDAP remains an external dependency addressable through `LLDAP_URL`; it does not need to be co-located in the same Compose stack.
* Adds one backend endpoint (`GET /api/v1/users/{uid}/profile`).
* New dependency: `github.com/go-ldap/ldap/v3` (sync module only).
* ~25 unit tests (backend client, LDAP client, worker logic, ordering).
* Zero changes to existing backend logic — consumes existing intent and shadow credential APIs.
