## Rationale

The spec defines provisioning as a Backend-to-Sync-Service push contract. This change implements the Backend side: structured intent persistence, LLDAP group name computation, and webhook integration. The Sync Service (Change 3) will consume these intents via the polling API. The intent table uses a four-state machine (pending/acknowledged/completed/failed) rather than the simpler two-state (received/processed) of webhook_events because the sync service is a separate process that needs intermediate acknowledgment to prevent double-pickup.

## Technical Specification

### 1. Provisioning Intents Table

`provisioning_intents` stores pending LLDAP mutations. The `UNIQUE(idempotency_key)` constraint prevents duplicate intents. The `status` column uses a CHECK constraint for the four-state machine. `webhook_event_id` is informational correlation to the originating webhook event, not a hard FK.

The backend pre-computes the `lldap_group` name so the sync service is a pure executor with no business logic.

### 2. Group Flattening

`FlattenLLDAPGroup(projectID, roleKey)` implements the `{project}_{role}` convention from the ldap-sync spec. Uses the stable, immutable project ID (not the mutable display name) so that project renames never orphan existing LLDAP group memberships or cause collisions between projects with similar display names. Both parts are lowercased. Example: "printing" + "member" → "printing_member" (P1 fix).

### 3. Intent Emission

`EmitProvisioningIntent` computes the flattened group directly from the stable project ID, persists with idempotency, and writes an audit log. The idempotency key format is `{action}:{targetUID}:{lldapGroup}:{webhookEventID}`. No project name resolution is needed — the group is derived from the immutable ID (P1 fix).

### 4. Webhook Integration

`processGrantAdded` and `processGrantRemoved` emit intents after their existing orchestration logic succeeds. Intent emission is non-fatal — a failure does not affect the webhook processing result. The `eventID` is threaded through process functions for correlation.

`user_deactivated` and `user_locked` do NOT emit intents in this change — full membership revocation requires the reconciliation loop in Change 4.

### 5. Sync Service API

Four endpoints for the sync service to consume intents:

- `POST /api/v1/intents/claim` — atomically claim pending intents (FIFO, configurable limit). Uses `FOR UPDATE SKIP LOCKED` so concurrent workers never claim the same intents (P2 fix). Returns claimed intents already transitioned to 'acknowledged'.
- `POST /api/v1/intents/{id}/complete` — mark as done
- `POST /api/v1/intents/{id}/fail` — record failure with error message

These use `withAPIKeyAuth` (shared MKAUTH_API_KEY), matching the internal service communication pattern.

- `GET /api/v1/intents` — operator view (uses `withUserAuth`)

### 6. Injectable Dependencies

All DB functions and service functions are injectable via `deps.go` for isolated testing.

## Verification

```bash
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./...  # 170 tests pass
```
