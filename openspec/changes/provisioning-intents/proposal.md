## Why

MkAuth had no mechanism to bridge Zitadel identity events to legacy infrastructure. When a user gained or lost a role in Zitadel, there was no way to propagate that change to LLDAP for Samba/UniFi access. The webhook handler processed grant events for Zitadel orchestration and cache invalidation, but the Bridge Plane described in the architecture design had no implementation.

## What Changes

* Adds a `provisioning_intents` table (migration 000009) with a four-state status machine (pending → acknowledged → completed | failed), idempotency deduplication, and pre-computed LLDAP group names.
* Adds `FlattenLLDAPGroup` utility implementing the `{project}_{role}` LLDAP group naming convention (lowercase, spaces → underscores).
* Adds `EmitProvisioningIntent` service that resolves project names, computes flattened groups, persists intents with idempotency, and writes audit logs.
* Integrates intent emission into webhook handlers: `grant_added`/`grant_changed` emit "add" intents, `grant_removed` emits "remove" intents. Intent failures are non-fatal to preserve existing webhook processing guarantees.
* Adds operator view (`GET /api/v1/intents`) and sync service API endpoints (`GET /api/v1/intents/pending`, `POST /api/v1/intents/{id}/acknowledge`, `POST /api/v1/intents/{id}/complete`, `POST /api/v1/intents/{id}/fail`) with API-key auth for internal service communication.

## Capabilities

### New Capabilities
* `provisioning-intents`: Structured intent emission on grant changes for downstream LLDAP sync.
* `lldap-group-flattening`: Project-prefixed group naming convention for LLDAP namespace translation.

### Modified Capabilities
* `webhook-dispatch`: Extended with provisioning intent emission after grant event processing.

## Impact

* Adds `provisioning_intents` table (migration 000009).
* Modifies `webhook.go` (thread eventID, emit intents from processGrantAdded/processGrantRemoved).
* Modifies `repositories.go` (6 new functions), `models.go` (1 new type), `deps.go` files (injectable vars), `router.go` (5 new routes).
* Creates `lldap.go` (group flattening), `provisioning.go` (intent emission), `handlers/intents.go` (5 handlers).
* 22 new tests (8 handler + 5 service LLDAP + 5 service provisioning + 4 webhook integration).
* Zero new go.mod dependencies.
