## 1. Database

- [x] 1.1 Create `000010_shadow_password_vault.up.sql` migration
- [x] 1.2 Create `000010_shadow_password_vault.down.sql` migration

## 2. Dependencies

- [x] 2.1 Add `golang.org/x/crypto` to `go.mod`

## 3. Models

- [x] 3.1 Add `ShadowCredential` struct to `models.go`
- [x] 3.2 Add `ShadowCredentialAudit` struct to `models.go`
- [x] 3.3 Add `ShadowCredentialStatus` struct to `models.go`

## 4. Repository

- [x] 4.1 Add `UpsertShadowCredential` with ON CONFLICT DO UPDATE
- [x] 4.2 Add `GetShadowCredential` (full row for sync service)
- [x] 4.3 Add `DeleteShadowCredential`
- [x] 4.4 Add `HasShadowCredential` (no hash returned)
- [x] 4.5 Add `InsertShadowCredentialAudit`
- [x] 4.6 Add `GetShadowCredentialAudit`

## 5. Service Layer

- [x] 5.1 Create `services/vault.go` with `ValidatePasswordComplexity`
- [x] 5.2 Add `hashPassword` (Argon2id PHC format)
- [x] 5.3 Add `SetShadowPassword`
- [x] 5.4 Add `ClearShadowPassword`
- [x] 5.5 Add vault injectable vars to `services/deps.go`

## 6. HTTP Handlers

- [x] 6.1 Create `handlers/vault.go` with `handleSetShadowCredential`
- [x] 6.2 Add `handleClearShadowCredential`
- [x] 6.3 Add `handleGetShadowCredentialStatus`
- [x] 6.4 Add `handleGetShadowCredentialHash` (sync service)
- [x] 6.5 Add `handleGetShadowCredentialAudit`
- [x] 6.6 Add self-only enforcement helper
- [x] 6.7 Add vault injectable vars to `handlers/deps.go`
- [x] 6.8 Add 5 routes to `router.go`

## 7. Service Tests

- [x] 7.1 `TestValidatePasswordComplexity_Valid`
- [x] 7.2 `TestValidatePasswordComplexity_TooShort`
- [x] 7.3 `TestValidatePasswordComplexity_NoUppercase`
- [x] 7.4 `TestValidatePasswordComplexity_NoLowercase`
- [x] 7.5 `TestValidatePasswordComplexity_NoDigit`
- [x] 7.6 `TestValidatePasswordComplexity_NoSymbol`
- [x] 7.7 `TestValidatePasswordComplexity_MultipleFailures`
- [x] 7.8 `TestSetShadowPassword_Success`
- [x] 7.9 `TestSetShadowPassword_ComplexityFailure_AuditsFailedValidation`
- [x] 7.10 `TestSetShadowPassword_RotationAuditsRotated`
- [x] 7.11 `TestSetShadowPassword_DBFailure`
- [x] 7.12 `TestClearShadowPassword_Success`
- [x] 7.13 `TestClearShadowPassword_NotFound`

## 8. Handler Tests

- [x] 8.1 `TestHandleSetShadowCredential_Success`
- [x] 8.2 `TestHandleSetShadowCredential_SelfOnly_Forbidden`
- [x] 8.3 `TestHandleSetShadowCredential_ValidationError`
- [x] 8.4 `TestHandleSetShadowCredential_MissingPassword`
- [x] 8.5 `TestHandleClearShadowCredential_Success`
- [x] 8.6 `TestHandleGetShadowCredentialStatus_HasCredential`
- [x] 8.7 `TestHandleGetShadowCredentialStatus_NoCredential`
- [x] 8.8 `TestHandleGetShadowCredentialHash_Success`
- [x] 8.9 `TestHandleGetShadowCredentialHash_NotFound`
- [x] 8.10 `TestHandleGetShadowCredentialAudit_Success`

## 9. Documentation

- [x] 9.1 Update `ROADMAP.md` — mark Shadow Password Vault as complete
- [x] 9.2 Update `feature-coverage.md` — update coverage matrix
