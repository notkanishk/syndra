# Zitadel Management Client — Implementation Record

**Phase:** 3 | **Status:** Complete | **Tests:** 22

## What Was Built
Direct HTTP client for Zitadel Management API v1 using JWT profile M2M auth. Zero new dependencies beyond existing `golang-jwt/jwt/v5`.

### Capabilities
- Service account key loading (PKCS1/PKCS8 fallback)
- JWT profile token exchange with thread-safe caching and 5-minute refresh margin
- `AddUserGrant`, `RemoveUserGrant`, `ListUserGrants`, `GetUser`
- Retry with exponential backoff on 429/503; auto token refresh on 401
- Graceful degradation to local-policy-only mode when credentials absent

### Design Decisions
- Direct HTTP over `zitadel-go` SDK (avoids ~30 transitive gRPC/protobuf deps)
- v1 Management API (stable, better documented for grant operations)
- Double-checked locking in token manager (prevents thundering herd)

## Key Files
- `backend/internal/zitadel/client.go` — HTTP management client
- `backend/internal/zitadel/token.go` — JWT assertion + token caching
- `backend/internal/zitadel/keyfile.go` — service account key loading
- `backend/internal/zitadel/orchestrator.go` — ZitadelClient interface
- `backend/internal/zitadel/deps.go` — injectable deps

## Verification
```bash
cd backend && go test ./internal/zitadel/... -v  # 22/22
cd backend && go test ./... && go vet ./...
```
