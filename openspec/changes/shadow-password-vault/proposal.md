## Why

MkAuth bridges Zitadel (OIDC) identity to legacy makerspace infrastructure — Samba file shares, UniFi network equipment, and other systems that authenticate via LLDAP. These systems require an LDAP-compatible password, but users authenticate to Zitadel with their own OIDC credentials that MkAuth never sees.

The Shadow Password Vault provides a **secondary, infrastructure-only credential** so that OIDC users can access Samba/LLDAP services without exposing or sharing their primary Zitadel password. This credential is narrowly scoped, independently auditable, and isolated from primary identity flows.

## What Changes

* Adds `shadow_credentials` table (migration 000010) — one active Argon2id-hashed credential per user, with upsert semantics and optional expiry.
* Adds `shadow_credential_audit` table — dedicated audit trail for all credential lifecycle events (set, rotated, cleared, failed_validation).
* Adds `ValidatePasswordComplexity` pure function — enforces 12+ character minimum with mixed case, digits, and symbols.
* Adds `SetShadowPassword` and `ClearShadowPassword` services with Argon2id hashing, byte zeroing, and audit emission.
* Adds 5 HTTP endpoints: user-facing set/clear/status/audit + sync-service hash retrieval.
* Adds `golang.org/x/crypto` dependency for `argon2.IDKey`.

## Capabilities

### New Capabilities

* `shadow-password-vault`: Users can set a secondary "Samba Access" password via the API. The credential is hashed with Argon2id (PHC string format), stored in a dedicated table, and exposed to the sync service for LLDAP `PasswordModify` operations.
* `infrastructure-credential-isolation`: Shadow credentials are stored, audited, and accessed through a separate path from the RBAC/identity system. The hash is never returned to user-facing endpoints.
* `credential-audit-trail`: Every credential operation is recorded with actor ID, action, IP address, and timestamp in a dedicated audit table.

### Modified Capabilities

* `provisioning-sync-api`: The sync service can now retrieve a user's shadow credential hash via `GET /api/v1/shadow-credentials/{uid}/hash` (API-key auth) when processing provisioning intents.

## Impact

* Adds `shadow_credentials` and `shadow_credential_audit` tables (migration 000010).
* Creates `services/vault.go`, `handlers/vault.go` and their test files.
* Modifies `models.go`, `repositories.go`, `services/deps.go`, `handlers/deps.go`, `handlers/router.go`.
* ~23 new tests (13 service + 10 handler).
* One new `go.mod` dependency: `golang.org/x/crypto`.
