## 1. Database

- [x] 1.1 Create `000009_provisioning_intents.up.sql` migration
- [x] 1.2 Create `000009_provisioning_intents.down.sql` migration

## 2. Models

- [x] 2.1 Add `ProvisioningIntent` struct to `models.go`

## 3. Repository

- [x] 3.1 Add `InsertProvisioningIntent` with ON CONFLICT DO NOTHING
- [x] 3.2 Add `ClaimPendingIntents` with FOR UPDATE SKIP LOCKED (atomic claim, P2 fix)
- [x] 3.3 Add `CompleteIntent` (acknowledged → completed)
- [x] 3.4 Add `FailIntent` (pending|acknowledged → failed)
- [x] 3.5 Add `GetProvisioningIntents` (operator view, optional status filter)

## 4. Service layer

- [x] 4.1 Create `services/lldap.go` with `FlattenLLDAPGroup`
- [x] 4.2 Create `services/provisioning.go` with `EmitProvisioningIntent`

## 5. Injectable deps

- [x] 5.1 Add provisioning intent vars to `services/deps.go`
- [x] 5.2 Add intent handler vars to `handlers/deps.go`

## 6. Webhook integration

- [x] 6.1 Thread `eventID` through process functions
- [x] 6.2 Emit "add" intent from `processGrantAdded`
- [x] 6.3 Emit "remove" intent from `processGrantRemoved`

## 7. HTTP handlers

- [x] 7.1 Create `handlers/intents.go` with 4 handlers (claim replaces pending+acknowledge, P2 fix)
- [x] 7.2 Add 4 routes to `router.go`

## 8. Tests

- [x] 8.1 `TestFlattenLLDAPGroup_BasicCase`
- [x] 8.2 `TestFlattenLLDAPGroup_AlreadyLowercase`
- [x] 8.3 `TestFlattenLLDAPGroup_MixedCaseProjectID`
- [x] 8.4 `TestFlattenLLDAPGroup_UnderscoresPreserved`
- [x] 8.5 `TestFlattenLLDAPGroup_ImmutableKeyStability` (P1 fix verification)
- [x] 8.6 `TestEmitProvisioningIntent_Success`
- [x] 8.7 `TestEmitProvisioningIntent_Duplicate`
- [x] 8.8 `TestEmitProvisioningIntent_StableGroupFromID` (P1 fix verification)
- [x] 8.9 `TestEmitProvisioningIntent_DBFailure`
- [x] 8.10 `TestEmitProvisioningIntent_AuditFailureNonFatal`
- [x] 8.11 `TestHandleGetProvisioningIntents_Success`
- [x] 8.12 `TestHandleGetProvisioningIntents_StatusFilter`
- [x] 8.13 `TestHandleGetProvisioningIntents_Empty`
- [x] 8.14 `TestHandleClaimIntents_Success` (P2 fix: atomic claim)
- [x] 8.15 `TestHandleClaimIntents_WithLimit`
- [x] 8.16 `TestHandleClaimIntents_Empty`
- [x] 8.17 `TestHandleCompleteIntent_Success`
- [x] 8.18 `TestHandleFailIntent_Success`
- [x] 8.19 `TestHandleFailIntent_NotFound`
- [x] 8.20 `TestWebhook_GrantAdded_EmitsAddIntent`
- [x] 8.21 `TestWebhook_GrantRemoved_EmitsRemoveIntent`
- [x] 8.22 `TestWebhook_GrantAdded_IntentFailureNonFatal`
- [x] 8.23 `TestWebhook_UserDeactivated_NoIntentEmitted`

## 9. P1/P2 fixes

- [x] 9.1 P1: Use stable project ID instead of mutable display name in `FlattenLLDAPGroup`
- [x] 9.2 P1: Remove `svcResolveProjectName` dependency from `EmitProvisioningIntent`
- [x] 9.3 P2: Replace `GetPendingIntents` + `AcknowledgeIntent` with atomic `ClaimPendingIntents` (FOR UPDATE SKIP LOCKED)
- [x] 9.4 P2: Replace `GET .../pending` + `POST .../acknowledge` with single `POST /api/v1/intents/claim`
