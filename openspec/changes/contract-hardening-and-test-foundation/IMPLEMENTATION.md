# Contract Hardening & Test Foundation — Implementation Record

**Phase:** 2 | **Status:** Complete | **Tests:** 82 (up from ~34)

## What Was Built
Strict request decoding (`decodeJSONStrict`) on all mutation endpoints. Required-field, enum, duration, and idempotency guards. Injectable-dependency pattern across all handlers and services. Database constraints in migrations 004 and 006.

## Risk Mitigations Achieved
- **Transport validation:** Unknown fields rejected on all mutations
- **Type ownership:** Purpose-built request DTOs for all mutation endpoints
- **Persistence invariants:** Status enums, positive durations, version bounds, resolution consistency, format_type enums, blank-name prevention, expiry-after-create
- **Authorization boundary:** `withUserAuth` middleware validates Zitadel JWTs in production
- **Regression coverage:** 82 tests covering all critical mutation endpoints

## Key Files
- `backend/internal/handlers/deps.go` — injectable dependency pattern
- `backend/internal/services/deps.go` — service-layer injectable deps
- `backend/db/migrations/000004_contract_hardening.up.sql`
- `backend/db/migrations/000006_additional_constraints.up.sql`
- `backend/internal/handlers/*_test.go` — handler test suite

## Verification
```bash
cd backend && go test ./... && go vet ./...
```
