# Shadow Password Vault — Implementation Record

**Phase:** 4 | **Status:** Complete | **Tests:** 23

## What Was Built
Infrastructure-only credential vault for Samba/LLDAP access. Argon2id-hashed, self-service API, dedicated audit trail, sync service integration.

### Capabilities
- `shadow_credentials` table — one active Argon2id credential per user (upsert semantics)
- `shadow_credential_audit` table — dedicated lifecycle audit (set/rotated/cleared/failed_validation)
- Password complexity validation (12+ chars, mixed case, digits, symbols)
- User-facing API: set/clear/status/audit (self-only enforcement)
- Sync service API: `GET /api/v1/shadow-credentials/{uid}/hash` (API-key auth)

### Security
- Argon2id: time=1, memory=64 MB, threads=4, keyLen=32, saltLen=16
- PHC string format (`$argon2id$v=19$...`) — self-describing, no separate parameter columns
- Password `[]byte` zeroed after hashing
- Hash never returned to user-facing endpoints

### Open Research
LLDAP password propagation compatibility is unverified. The vault is implemented and auditable but not yet validated for end-to-end LLDAP sync.

## Key Files
- `backend/internal/services/vault.go` — Argon2id hashing, complexity validation
- `backend/internal/handlers/vault.go` — HTTP handlers with self-only enforcement
- `backend/db/migrations/000010_shadow_password_vault.up.sql`

## Verification
```bash
cd backend && go test ./... && go vet ./...
```
