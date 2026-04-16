## 1. Project Scaffolding

- [x] 1.1 Create `sync/go.mod` with `mkauth-sync` module
- [x] 1.2 Create `sync/cmd/sync/main.go` entry point
- [x] 1.3 Create `sync/Dockerfile` (multi-stage build)
- [x] 1.4 Create `sync/internal/config/config.go` with `Load()`

## 2. Backend HTTP Client

- [x] 2.1 Create `sync/internal/backend/types.go` with shared types
- [x] 2.2 Create `sync/internal/backend/client.go` with `NewClient`
- [x] 2.3 Implement `ClaimIntents`
- [x] 2.4 Implement `CompleteIntent`
- [x] 2.5 Implement `FailIntent`
- [x] 2.6 Implement `GetShadowCredentialHash` (404 → empty, nil)
- [x] 2.7 Implement `GetUserProfile`

## 3. LLDAP Client

- [x] 3.1 Create `sync/internal/ldap/client.go` with `Connect` and `Close`
- [x] 3.2 Implement `withConn` auto-reconnect pattern
- [x] 3.3 Implement `EnsureUser` (search + create/update)
- [x] 3.4 Implement `EnsureGroup` (search + create)
- [x] 3.5 Implement `AddUserToGroup` (idempotent, member attr on group)
- [x] 3.6 Implement `RemoveUserFromGroup` (idempotent)
- [x] 3.7 Implement `SetUserPassword` (LDAP Modify userPassword)

## 4. Per-UID Ordering

- [x] 4.1 Create `sync/internal/worker/ordering.go` with `UIDLocker`
- [x] 4.2 Implement Lock/Unlock with waiter-based cleanup

## 5. Worker Pool

- [x] 5.1 Create `sync/internal/worker/worker.go` with `Run`
- [x] 5.2 Implement polling loop with ticker
- [x] 5.3 Implement worker goroutine pool
- [x] 5.4 Implement `processIntent` with per-UID locking and EnsureUser/Group
- [x] 5.5 Implement `syncShadowPassword` with byte zeroing
- [x] 5.6 Implement `retryTransient` with exponential backoff
- [x] 5.7 Implement `isTransientError` LDAP error classification

## 6. Backend: User Profile Endpoint

- [x] 6.1 Add `handleGetUserProfile` handler
- [x] 6.2 Add route `GET /api/v1/users/{uid}/profile` with `withAPIKeyAuth`
- [x] 6.3 Add injectable dep for user profile lookup

## 7. Docker Integration

- [x] 7.1 Add sync service to `docker-compose.yml`

## 8. Backend Client Tests

- [x] 8.1 `TestClaimIntents_Success`
- [x] 8.2 `TestClaimIntents_Empty`
- [x] 8.3 `TestClaimIntents_ServerError`
- [x] 8.4 `TestCompleteIntent_Success`
- [x] 8.5 `TestFailIntent_Success`
- [x] 8.6 `TestGetShadowCredentialHash_Found`
- [x] 8.7 `TestGetShadowCredentialHash_NotFound`
- [x] 8.8 `TestGetUserProfile_Success`
- [x] 8.9 `TestAuthorizationHeader`

## 9. LDAP Client Tests

- [x] 9.1 `TestUserDN`
- [x] 9.2 `TestGroupDN`
- [x] 9.3 `TestUserDN_SpecialCharacters`
- [x] 9.4 `TestIsConnectionError`

## 10. Ordering Tests

- [x] 10.1 `TestUIDLocker_DifferentUIDs_Concurrent`
- [x] 10.2 `TestUIDLocker_SameUID_Sequential`
- [x] 10.3 `TestUIDLocker_MapCleanup`

## 11. Worker Tests

- [x] 11.1 `TestProcessIntent_AddSuccess`
- [x] 11.2 `TestProcessIntent_RemoveSuccess`
- [x] 11.3 `TestProcessIntent_LDAPPermanentFailure`
- [x] 11.4 `TestProcessIntent_LDAPTransientRetryThenSuccess`
- [x] 11.5 `TestProcessIntent_ShadowPasswordSync`
- [x] 11.6 `TestProcessIntent_ShadowPasswordAbsent`
- [x] 11.7 `TestRetryTransient_PermanentError`
- [x] 11.8 `TestRetryTransient_ContextCancelled`
- [x] 11.9 `TestPoll_ClaimsAndDispatches`

## 12. Documentation

- [x] 12.1 Update `ROADMAP.md` — mark Sync Service as complete
- [x] 12.2 Update `feature-coverage.md`

## 13. Follow-up Research

- [ ] 13.1 Verify how the real target LLDAP deployment expects password updates to be performed
- [ ] 13.2 Verify whether the target LLDAP deployment accepts MkAuth's stored pre-hashed Argon2id credential format
- [ ] 13.3 Verify compatibility specifically against the external Proxmox LXC LLDAP installation
- [ ] 13.4 Decide whether the current `SetUserPassword` design is valid, needs revision, or should be replaced with a different password-sync model
