# Contract 1 inventory (10 Nov 2026)

The wire contract between the backend and the TrueNAS add-on as it stood before this change, mapped from code. Slice 0 pins exactly this.


Facts only, file:line. BE = backend, AO = add-on (addons/truenas). Lenient = json.Unmarshal; strict = decodeStrict (DisallowUnknownFields).

## Summary
1. 12 add-on routes (addons/truenas/server.go:68-93), not the 8 addons/contract/README.md describes; /reach, /settings, /homes and PUT /settings/{name} are missing from the README.
2. Four golden fixtures (apply_request, operation_request, plan_request, apply_response) pin only those bodies. Capabilities (manifest), health, subjects, values, reach, settings, homes, lifecycle and the plan reply have no fixture. A fifth fixture (transport_derivation.json) pins key derivation + request signing.
3. contract_version is checked on every mutating body except PUT /settings/{name}. The manifest is the only response the BE checks for version, and it decodes strictly; every other BE response decode is lenient.
4. Field names group, home, home_quota, enabled, smb_enabled are hardcoded across BE handlers/services and UI. TrueNAS-specific logic sits in services/entitlement.go (SMB as a lifecycle field) and an smb:// builder at backend/internal/addons/read.go:781.
5. Idempotency travels two ways: body call_id (AO reads it for apply and operations) and Idempotency-Key header (AO reads it only for settings; BE sends it on every write). Signing covers method, path and body only (BE transport.go:441, AO transport.go:173).

## Endpoints
| # | Route | BE caller | AO handler | Types BE / AO | Decode BE / AO | Fixture | Idempotency | contract_version |
|---|---|---|---|---|---|---|---|---|
| 1 | GET /capabilities | registry.go:722 httpFetchManifest | server.go:103 | Manifest manifest.go:45 / Manifest capabilities.go:19 | strict / encoder | none | n/a | in response; checked registry.go:389 (equality) |
| 2 | GET /health | read.go:396 readHealth | server.go:224 | TargetHealth read.go:326 / Health server.go:177 | lenient | none | n/a | none |
| 3 | GET /subjects | read.go:291 Subjects | server.go:321 | subjectsEnvelope read.go:273 (+TargetAccount read.go:204) / SubjectsResponse server.go:279 (+SubjectView, Subject store.go:194) | lenient | none | n/a | none |
| 4 | GET /values/{field} | read.go:494 ResolvesValue; read.go:586 ReadValues | capabilities.go:230 | anon struct read.go:503 / ValuesResponse capabilities.go:209 | lenient | none | n/a | none |
| 5 | GET /reach | read.go:725 ReadReach | reach.go:394 | Reach read.go:708 (+Place read.go:741) / ReachResponse reach.go:379 (+Place reach.go:39) | lenient | none | n/a | none |
| 6 | GET /settings | read.go:620 ReadSettings | settings.go:66 | TargetSetting read.go:606 / SettingView settings.go:28 | lenient | none | n/a | none |
| 7 | GET /homes | read.go:692 ReadHomes | home.go:242 | Homes read.go:684 / HomesResponse home.go:235 | lenient | none | n/a | none |
| 8 | PUT /settings/{name} | read.go:656 PutSetting (inline map body read.go:650) | settings.go:83 | inline map / putSettingRequest settings.go:75; resp SettingView | none / strict | none | Idempotency-Key = addon_operations row id; AO dedupes (kindSetting) | none either side |
| 9 | POST /plan | read.go:152 Plan; callers entitlement_rehearsal.go:129, drift/addon.go:408, propagation/addon.go:363 | plan.go:75 | planEnvelope read.go:107 (+planSubjectWire, PlanSubject read.go:39) / PlanRequest plan.go:38; resp PlanResult read.go:82 (+SubjectOutcome read.go:53, BindingConflict read.go:70) / PlanResponse plan.go:48 (+ApplyOutcome apply.go:65) | lenient / strict | request: plan_request.json (BE contract_test.go:116; AO contract_test.go:80,97). Response: none | not a write | body; AO checks plan.go:81 |
| 10 | POST /apply | entitlement.go:161 Apply; caller propagation/addon.go:170 | apply.go:322 | ApplyRequest entitlement.go:33 (applyEnvelope :109) / ApplyRequest apply.go:30; resp ApplyResponse entitlement.go:60 / ApplyOutcome apply.go:65 | lenient / strict | apply_request.json + apply_response.json (both sides) | body call_id (AO Recall apply.go:351, Remember 2xx only :413); header sent but not read by AO | body; AO checks apply.go:328 |
| 11 | POST /operations/{name} | transport.go:237 Call (addonop/dispatch.go:307); read ops read.go:815 (storage.status), :912 (activity.get), :1030 (health.get) | operations.go:83 | CallRequest transport.go:84 (callEnvelope :167) / OperationRequest operations.go:29; resp CallResponse / OperationResult operations.go:56 (+ActivityReport, StorageStatus, TargetHealth) | lenient per op / strict | operation_request.json (both). Response: none | body call_id, mutating ops only remembered; header sent not read | body; AO checks operations.go:90 |
| 12 | POST /lifecycle | read.go:440 SetLifecycle (via operate pipe.go:101) | lifecycle.go:172 | inline map read.go:427 / LifecycleRequest lifecycle.go:157; resp inline maps | lenient / strict | none | header sent (operate row id), not read by AO; no call_id | body; AO checks lifecycle.go:178; reason required |

Lifecycle refusal: 503 + Retry-After, error LIFECYCLE_REFUSAL (AO lifecycle.go:143-150), BE classifyStatus transport.go:390-420.

## Manifest (GET /capabilities)
BE addons/manifest.go / AO addons/truenas/capabilities.go. The manifest is a ceiling intersected with the BE's operationPolicy (policy.go:52-210).
- contract_version int: equality check, mismatch refused (registry.go:389, ErrUnsupportedContract registry.go:104). ContractVersion = 1 (BE manifest.go:18, AO transport.go:40).
- product, product_version: decoded, product unread; health carries its own product_version.
- entitlement_schema []EntitlementField {name, type ("string","string[]","bool","int"), lifecycle bool, label, single, per_person}: read by mappings.go:228,306,325,336; mappable_values.go:37,48,62; allowances.go:159,209,215,222,228.
- operations []Operation {id, scope (member|admin), confirm, secret_params, available, unavailable_reason}: policy.go:179-305, registry.go:555,671-695, dispatch.go:178,214,255, redact.go:33, targets.go:263.
- connection {protocol, host}: my_storage.go:295-311 builds the member's address; read.go:772-781 builds smb://.

## Field names hardcoded outside the add-on (non-test)
BE: handlers/my_storage.go:330 (group), :386 (home), :473 (lifecycle states); person_storage.go:226 (group); target_homes.go:23 const fieldHomeQuota="home_quota", :97 (home), :98,109,167; mapping_plan.go:174 (field=="group" wording); mappings.go:325 IsLifecycleField, :336 (f.Type=="int"); services/entitlement.go:110-111 FieldEnabled/FieldSMBEnabled, :116 IsLifecycleField, :282,:344,:472,:511 (Desired writes enabled+smb_enabled), :260-262,289-295 (disabling takes SMB with it); services/role_members.go:306 ("group is the sole entitlement field"); drift/addon.go:723.
UI: lib/role-mapping-plan.ts:23-25 FIELD_GROUP/FIELD_HOME/FIELD_HOME_QUOTA (+ :29,42-44,72-74,138); targets/HomeLimit.tsx:33 FIELD="home_quota"; AddMappingDialog.tsx:361 FIELD="group", :403-416 copy about enabled/smb_enabled; MappingManagement.tsx:91,132-224 group-only branches; PeopleOnTarget.tsx:198; storage/MyStorage.tsx:104,256-273,460; ui/Withheld.tsx:51,105,176; review/HoldsDueForReview.tsx:107,109; people/PersonStorage.tsx:131,513,524; queries/useMyStorage.ts:130 smb_enabled type.

## Vocabulary duplicated on both sides
- Effects: AO apply.go:174-177 (applied, no_change, blocked, apply). BE services/bulk.go:48-59 (+failed, queued), db/plans.go:56-58 (plan effects, validated :71,115), addons/entitlement.go:102 effectBlocked. UI PlanReview/RehearsalDialog/ActionOutcome.
- Transport outcomes BE-only: transport.go:29-50 (succeeded, rejected, unreached, indeterminate).
- Error codes, AO definitions: PLAN_STALE apply.go:393 (BE entitlement.go:100 CodePlanStale); CONTRACT_VERSION_MISMATCH server.go:397; FINGERPRINT_REQUIRED apply.go:341; CALL_ID_REUSED server.go:409; OPERATION_MISMATCH operations.go:104; TARGET_VERSION_UNSUPPORTED apply.go:380; TARGET_UNREADABLE (plan, home, reach, settings, capabilities); STORE_UNREADABLE; NO_CURRENT_READ server.go:343; REACH_NEVER_READ reach.go:402; SETTINGS_UNREADABLE; NO_SUCH_SETTING; INVALID_BODY; NOT_A_HOME_PLACE; SETTING_NOT_SAVED; LIFECYCLE_REFUSAL; REASON_REQUIRED; UNKNOWN_LIFECYCLE_STATE; UNKNOWN_FIELD capabilities.go:276; SCOPE_ACKNOWLEDGEMENT_REQUIRED plan.go:89; OPERATION_FAILED; APPLY_FAILED; UNAUTHENTICATED transport.go:200; BAD_REQUEST. Most have no BE constant; BE matches PLAN_STALE only.
- Operation ids: AO capabilities.go:127-178 + operations.go:127-141; BE policy.go:52-185 + literals read.go:436 (lifecycle.set), :652 (setting.put), :803 (storage.status), :899 (activity.get), :1019 (health.get).
- Scope strings, secret param names (password, elevated_key), idempotency kinds (AO server.go:355-358), lifecycle states (active, draining, read_only), key-expiry strings, place access (read, change), signature header X-Syndra-Addon-Signature (BE transport.go:71, AO transport.go:35), Idempotency-Key (BE pipe.go:36, AO settings.go:98), derivation info strings (derive.go:32-33 both).

## Transport and auth
Per-target secret -> HKDF-derived Ed25519 TLS key (BE pins the public key) and HMAC request-signing key (method, path, body, timestamp; 2 min tolerance). AO: transport.go:35-200, derive.go:32-72, main.go:110-444 (ADDON_SECRET, *_FILE). BE: transport.go:71-441, pipe.go:36-101, derive.go:21-138, credentials.go, registry.go:32-178 (ADDON_<TARGET>_SECRET[_FILE]). Fixture transport_derivation.json (targets truenas and unifi already present), asserted by AO derive_test.go:61,141 and BE derive_test.go:66,106,157 and crossbinary_test.go:41 (real add-on binary vs client).

## Backend assumptions that the add-on is TrueNAS (non-test code)
read.go:772-781 Place.Address builds smb://; read.go:755-759 SMBAddress/UNCAddress; read.go:541-553 SMB follows password; services/entitlement.go SMB as lifecycle field (87-88,96,111,117,260-295,512); role_members.go:306-313 group as sole field; many comments only. policy.go:48-50 says the operation policy is target-neutral by design.

## Observed mismatches
- BE ActivityEvent has Detail (read.go:875); AO ActivityEvent (operations.go:505-516) has no detail.
- AO Health.CheckedAt (checked_at) has no BE counterpart.
- LogRecords: AO uint64, BE int64.
- PUT /settings/{name} carries no contract_version.
- AO reads Idempotency-Key only for settings; apply/operations dedupe on body call_id.
- READMEs say eight routes; AO registers 12.
- BE decodes all responses leniently except the manifest (strict).
