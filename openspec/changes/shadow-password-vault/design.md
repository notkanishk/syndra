## Rationale

The LDAP sync spec defines the Shadow Password as an infrastructure-only credential bridge: it must never be presented as the user's primary account password, must be independently auditable, and must be narrowly scoped to Samba/LLDAP access. This design implements the vault as a thin, isolated layer that stores exactly one Argon2id-hashed credential per user, exposes it only to the internal sync service, and maintains a dedicated audit trail separate from the RBAC audit logs.

Key design choices:
- **Argon2id over bcrypt**: Better resistance to side-channel and GPU attacks, tunable memory parameter (64 MB) that makes parallel cracking expensive.
- **PHC string format**: `$argon2id$v=19$m=65536,t=1,p=4$<salt>$<key>` — self-describing, standard, no separate column needed to reconstruct parameters.
- **One credential per user**: `UNIQUE(user_id)` + upsert. The audit table tracks the full history; the credentials table holds only the current state.
- **No provisioning intents for passwords**: Provisioning intents model group membership mutations. Password sync is a different concern with a different lifecycle — the sync service checks for a shadow credential when processing any user intent.

## Technical Specification

### 1. Database Schema (Migration 000010)

**`shadow_credentials`** — one active credential per user:

```sql
CREATE TABLE IF NOT EXISTS shadow_credentials (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         TEXT        NOT NULL UNIQUE CHECK (btrim(user_id) <> ''),
    credential_hash TEXT        NOT NULL CHECK (btrim(credential_hash) <> ''),
    algorithm       TEXT        NOT NULL DEFAULT 'argon2id' CHECK (algorithm IN ('argon2id')),
    salt_params     TEXT        NOT NULL CHECK (btrim(salt_params) <> ''),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ
);
```

Upsert uses `ON CONFLICT (user_id) DO UPDATE SET credential_hash, algorithm, salt_params, updated_at = NOW(), rotated_at = NOW()`.

**`shadow_credential_audit`** — dedicated audit trail:

```sql
CREATE TABLE IF NOT EXISTS shadow_credential_audit (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    TEXT        NOT NULL CHECK (btrim(user_id) <> ''),
    action     TEXT        NOT NULL CHECK (action IN ('set', 'rotated', 'cleared', 'failed_validation')),
    actor_id   TEXT        NOT NULL CHECK (btrim(actor_id) <> ''),
    ip_address TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 2. Models

Three structs appended to `models.go`:
- `ShadowCredential` — full DB row. `credential_hash` tagged `omitempty` to prevent accidental JSON exposure.
- `ShadowCredentialAudit` — audit row.
- `ShadowCredentialStatus` — user-facing view: `has_credential bool` + timestamps, no hash.

### 3. Repository Functions

Six functions appended to `repositories.go`, following existing patterns (pgx, `fmt.Errorf` wrapping, `pgx.ErrNoRows` handling):

1. `UpsertShadowCredential(ctx, userID, hash, algorithm, saltParams) (string, error)` — `RETURNING id`.
2. `GetShadowCredential(ctx, userID) (ShadowCredential, error)` — full row for sync service.
3. `DeleteShadowCredential(ctx, userID) error` — checks `RowsAffected`.
4. `HasShadowCredential(ctx, userID) (ShadowCredentialStatus, error)` — excludes `credential_hash` from SELECT.
5. `InsertShadowCredentialAudit(ctx, userID, action, actorID, ipAddress) error` — simple insert.
6. `GetShadowCredentialAudit(ctx, userID) ([]ShadowCredentialAudit, error)` — ordered by `created_at DESC`.

### 4. Service Layer (`services/vault.go`)

**Argon2id constants**: time=1, memory=64 MB, threads=4, keyLen=32, saltLen=16.

**`ValidatePasswordComplexity(password string) error`** — pure function:
- Length >= 12 characters.
- At least one uppercase, lowercase, digit, and symbol.
- Returns all failing requirements in a single error message.

**`hashPassword(plaintext string) (hash, saltParams string, err error)`**:
- Generates 16-byte `crypto/rand` salt.
- Calls `argon2.IDKey`.
- Encodes as PHC string: `$argon2id$v=19$m=65536,t=1,p=4$<base64-salt>$<base64-key>`.
- Zeros the `[]byte` copy of plaintext after hashing.

**`SetShadowPassword(ctx, userID, actorID, plaintext, ipAddress string) error`**:
1. Validate complexity → on failure, audit `failed_validation`, return error.
2. Check if credential exists (`HasShadowCredential`) → determines action: `set` vs `rotated`.
3. Hash with Argon2id.
4. `UpsertShadowCredential`.
5. Audit the action.

**`ClearShadowPassword(ctx, userID, actorID, ipAddress string) error`**:
1. `DeleteShadowCredential`.
2. Audit `cleared`.

Injectable deps added to `services/deps.go`.

### 5. HTTP Handlers (`handlers/vault.go`)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| PUT | `/api/v1/users/{uid}/shadow-credential` | `withUserAuth` | Set password (self-only) |
| DELETE | `/api/v1/users/{uid}/shadow-credential` | `withUserAuth` | Clear password (self-only) |
| GET | `/api/v1/users/{uid}/shadow-credential/status` | `withUserAuth` | Has credential? (no hash) |
| GET | `/api/v1/users/{uid}/shadow-credential/audit` | `withUserAuth` | Audit trail |
| GET | `/api/v1/shadow-credentials/{uid}/hash` | `withAPIKeyAuth` | Sync service hash retrieval |

**Self-only enforcement**: `getAdminUserID(ctx)` must match `{uid}`. In dev mode (empty actor from API-key auth), self-check is skipped.

Injectable deps added to `handlers/deps.go`. Routes registered in `router.go`.

### 6. Security

- Argon2id with 64 MB memory cost makes parallel cracking expensive.
- Password `[]byte` zeroed after hashing (Go string immutability is a known limitation — the byte copy window is minimized).
- `credential_hash` is never returned to user-facing endpoints. Only `GET /shadow-credentials/{uid}/hash` (API-key auth) returns it.
- Self-only enforcement prevents one user from managing another's credential.
- All operations audited with actor, action, IP, and timestamp.

### 7. Sync Service Integration

Password changes do NOT emit provisioning intents. The sync service checks `GET /shadow-credentials/{uid}/hash` when processing any provisioning intent for a user, and transmits the hash via LDAP `PasswordModify` over LDAPS.

## Verification

```bash
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./...
```
