## 1. Service account key loading

- [x] 1.1 Create `ServiceAccountKey` struct mirroring Zitadel machine user key file format
- [x] 1.2 Implement `LoadServiceAccountKey` with type validation, field validation, PEM decode, PKCS1/PKCS8 fallback
- [x] 1.3 Write 5 key file tests: valid load, missing fields, invalid PEM, wrong type, file not found

## 2. Token manager

- [x] 2.1 Implement JWT assertion builder (RS256, iss/sub/aud/kid/exp claims) using `golang-jwt/jwt/v5`
- [x] 2.2 Implement token exchange via `POST /oauth/v2/token` with `jwt-bearer` grant type
- [x] 2.3 Implement thread-safe token caching with 5-minute refresh margin and double-checked locking
- [x] 2.4 Implement `ForceRefresh()` for 401-triggered cache invalidation
- [x] 2.5 Write 4 token manager tests: cached token, expired refresh, refresh failure, JWT assertion format validation

## 3. Interface and types

- [x] 3.1 Extend `ZitadelClient` interface: add `RemoveUserGrant`, `ListUserGrants`, `GetUser`
- [x] 3.2 Add `UserGrant` and `ZitadelUser` response types
- [x] 3.3 Change `MgmtClient` from `interface{}` to typed `ZitadelClient`
- [x] 3.4 Remove runtime type assertions from `EnforceMappingRules` and `AssignUserToRole`

## 4. HTTP management client

- [x] 4.1 Implement `doRequest` with Bearer token injection, JSON marshalling, response status handling
- [x] 4.2 Implement 401 → token refresh → retry-once logic
- [x] 4.3 Implement exponential backoff retry on 429/503 (100ms, 200ms, 400ms, max 3 attempts)
- [x] 4.4 Implement `AddUserGrant`: `POST /management/v1/users/{userId}/grants`
- [x] 4.5 Implement `RemoveUserGrant`: `DELETE /management/v1/users/{userId}/grants/{grantId}`
- [x] 4.6 Implement `ListUserGrants`: `POST /management/v1/users/grants/_search` with `userIdQuery` filter body
- [x] 4.7 Implement `GetUser`: `GET /management/v1/users/{userId}` with nested `human.profile`/`human.email` parsing
- [x] 4.8 Wire up `InitClient` to create real client when credentials are present

## 5. Injectable dependencies

- [x] 5.1 Create `deps.go` with `httpDo`, `timeNow`, `tokenHTTPClient`, `dbGetActiveMappingRules`
- [x] 5.2 Refactor `orchestrator.go` to call `dbGetActiveMappingRules` through injectable var

## 6. API client tests

- [x] 6.1 `TestAddUserGrant_Success` — verify request body and path
- [x] 6.2 `TestAddUserGrant_401_RefreshesToken` — mock 401 → refresh → retry 200
- [x] 6.3 `TestRemoveUserGrant_Success` — verify DELETE to correct path
- [x] 6.4 `TestListUserGrants_Success` — verify correct endpoint path and `userIdQuery` filter in body
- [x] 6.5 `TestGetUser_Success` — verify nested Zitadel v1 response parsing (human.profile.displayName, human.email.email)
- [x] 6.6 `TestDoRequest_Retries429` — verify exponential backoff
- [x] 6.7 `TestDoRequest_NoRetryOn400` — verify single attempt on client errors

## 7. Orchestrator tests

- [x] 7.1 `TestEnforceMappingRules_NilClientGraceful` — no panic when client absent
- [x] 7.2 `TestEnforceMappingRules_MatchingRule` — verify `AddUserGrant` called with correct args
- [x] 7.3 `TestEnforceMappingRules_GrantErrorContinues` — both rules attempted despite first failing
- [x] 7.4 `TestAssignUserToRole_Success` — verify single grant call
- [x] 7.5 `TestAssignUserToRole_NilClient` — error returned

## 8. Documentation and environment

- [x] 8.1 Add `ZITADEL_MACHINE_KEY_PATH` to `.env.example`
- [x] 8.2 Update `AUDIT.md` zitadel section from stub to implemented
- [x] 8.3 Check off Management Client in `ROADMAP.md`
- [x] 8.4 Update `feature-coverage.md`: M2M status from Partial to Integrated
- [x] 8.5 Update `design.md` Zitadel integration status
- [x] 8.6 Update `contract-hardening-and-test-foundation/design.md` next-step reference

## 9. P1 fixes (contract mismatches)

- [x] 9.1 Fix `ListUserGrants` endpoint: `POST /management/v1/users/grants/_search` with `userIdQuery` filter (was incorrectly calling user-scoped `/{userId}/grants/_search` with empty body)
- [x] 9.2 Fix `GetUser` response parsing: extract `displayName` from `user.human.profile.displayName` and `email` from `user.human.email.email` (was incorrectly assuming flat fields on `user`)
- [x] 9.3 Update `TestListUserGrants_Success` to verify correct path and query filter structure
- [x] 9.4 Update `TestGetUser_Success` to use real Zitadel v1 nested response shape
