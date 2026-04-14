# Contract Hardening and Test Foundation Design

## 1. Goal
MkAuth needs a contract backbone that is explicit, validated, testable, and difficult to misuse. This initiative is not about adding new user-facing features first. It is about making the existing and planned features safe enough to trust as the system grows into a real identity and authorization control plane.

## 2. Quality Principles

### 2.1 Backend contracts are authoritative
The backend must be the final authority for request validity, authorization, business-rule enforcement, and persistence invariants. UI checks may improve usability, but they do not count as security controls.

### 2.2 Types are purpose-built, not incidental
Transport DTOs, domain entities, persistence rows, and UI view models must not be treated as interchangeable just because they share field names. Each layer should express its own intent and constraints.

### 2.3 Validation is layered
Every critical mutation path should be validated in four layers:

```text
HTTP request shape
  -> semantic field validation
  -> business-rule validation
  -> database invariants
```

### 2.4 Unknown or ambiguous input is rejected
Permissive decoding creates hidden attack and regression surface. Request decoding should reject unknown fields, invalid enums, impossible dates, empty identifiers, and inconsistent state transitions.

### 2.5 Documentation is part of the contract
If the implemented contract changes, the corresponding OpenSpec documents, roadmap, and coverage matrix must change in the same workstream.

### 2.6 Zitadel communication is standardized around Actions v2
Zitadel is the external source-of-truth boundary, so compatibility and security at that edge must be designed around Zitadel Actions v2 behavior and constraints. Internal contracts between the frontend UI, backend, and LLDAP sync service may be purpose-built for MkAuth, but they must not weaken or redefine the Zitadel-facing contract.

## 3. Risk Summary

### 3.1 Transport validation risk — MITIGATED
All mutation handlers use `decodeJSONStrict` which rejects unknown fields. Required-field checks, enum validation (status, format_type), negative duration rejection, and whitespace-only name rejection are implemented and covered by handler tests. Remaining gap: unbounded string lengths not constrained at the HTTP layer (low priority; DB constraints provide a backstop).

### 3.2 Type ownership risk — PARTIALLY MITIGATED
Purpose-built request DTOs exist for all mutation endpoints. Domain models are distinct from transport types. Persistence models are implicitly shaped by repository queries. Formal persistence struct types (separate from domain models) have not been added — this remains a future cleanup if the schema diverges significantly from domain.

### 3.3 Persistence invariant risk — MITIGATED
Migrations 004 and 006 together enforce: `status IN ('pending','approved','rejected')`, `duration_days > 0` or null, `version > 0`, `resolved_at` consistency, `format_type` enum, blank-name prevention on bundles and bundle roles, and expiry-after-create on direct grants. Critical domain rules are enforced below the application layer.

### 3.4 Authorization boundary risk — MITIGATED
Backend has `withUserAuth` middleware that validates Zitadel-issued RS256 JWTs (JWKS-backed) in production and falls back to API key for local dev. Admin user ID is extracted from context and written to audit logs. Frontend OIDC token forwarding is now implemented: the UI performs a PKCE authorization code flow against Zitadel, stores the raw access token in the session cookie, and forwards it as `Authorization: Bearer <token>` on both proxy requests (`ui/src/app/api/proxy/[...path]/route.ts`) and SSR server component fetches (`ui/src/lib/api.ts`). When `ZITADEL_DOMAIN` is set the shared API key is no longer used as the primary authorization proof.

### 3.5 Regression risk — MITIGATED
82 backend tests now cover all critical mutation endpoints, service logic, claim formatting, lineage assembly, governance nil-safety, webhook validation, action injection, and onboarding flows. Injectable dependency pattern is established across all handler and service layers.

### 3.6 External contract drift risk — MITIGATED (by documentation)
Zitadel Actions v2 is explicitly documented as the external boundary in the architecture design and contract-quality spec. Internal contracts (FE→BE, BE→Sync) are separately defined. This rule is enforced by the documentation update requirement in section 8.

## 4. Target Architecture for Contracts

### 4.1 Layered model taxonomy
The hardened codebase should distinguish at least these classes of types:
* request DTOs: exact JSON contract for inbound API payloads
* response DTOs: exact JSON contract for outbound API payloads
* domain models: validated business concepts with bounded values
* persistence models: DB read/write shapes and query-layer records
* UI view models: frontend-only composition types

### 4.2 Bounded values become explicit types
The following values should be represented as explicit enums/constants or equivalent bounded types instead of free-form strings:
* access request status
* claim format type
* topology node kind
* topology edge kind
* audit action classes where practical
* role-reason kind
* session role

### 4.3 Database invariants mirror domain invariants
Critical rules should be enforced in Postgres through constraints and indexes in addition to application validation.

Examples:
* `status IN ('pending', 'approved', 'rejected')`
* `duration_days IS NULL OR duration_days > 0`
* `version > 0`
* `resolved_at IS NOT NULL` when status is terminal
* no empty-string identifiers after trimming
* allowed `format_type` values only

### 4.4 Error contracts are stable
Validation failures should return stable error codes and predictable field-level detail so UI and automation behavior can remain deterministic.

### 4.5 External vs internal contract boundary
MkAuth should distinguish between:
* external source-of-truth contracts: Zitadel-facing communication, which must remain Actions v2-compatible and security-hardened
* internal control-plane contracts: frontend UI to backend and backend to sync-service payloads, which may be self-defined as long as they are explicit, validated, authenticated, and isolated from the external boundary

For privileged frontend-to-backend requests, the preferred production contract is simple: the frontend sends a Zitadel-issued user access token, the backend validates it, and the backend makes the authorization decision. A shared internal API key may remain as an internal service guard, but it must not be the primary authorization proof for admin mutations.

### 4.6 Zitadel integration responsibility split
MkAuth should use two different mechanisms for two different classes of work:
* Zitadel Actions v2: token-time claim shaping and Zitadel-native event-triggered compatibility paths
* service user account (machine-to-machine): backend-owned management operations against the Zitadel Management API

This means a service user account is still required for the app's control-plane responsibilities, but it should be narrowly scoped and never treated as a substitute for Actions v2 at the source-of-truth claim boundary.

## 5. Testing Strategy

### 5.1 Priority order
1. Backend validation and service logic
2. Backend handler and repository integration paths
3. Frontend proxy authorization and critical form flows
4. Broader UI rendering tests

### 5.2 Test categories
* decoder and validator unit tests
* handler tests with `httptest`
* service-layer table tests
* repository or DB integration tests for constraints and queries
* authorization boundary tests
* contract regression tests for claim payloads and topology responses
* frontend tests for proxy scoping and member/admin behavior

### 5.3 Minimum critical backend coverage
The first hardening wave should cover:
* bundle creation and assignment contracts
* mapping-rule creation and cycle detection
* direct grants and expiry handling
* access request create and resolve flows
* audit log response shape
* application claim simulation and action payload shaping
* webhook payload validation and safe failure behavior
* internal provisioning intent validation without diluting Actions v2 compatibility at the Zitadel edge

## 6. Validation and Test Matrix

### 6.1 Endpoint matrix

| Endpoint | Risk | Validation coverage | Test coverage | Status |
| --- | --- | --- | --- | --- |
| `POST /api/v1/bundles` | medium | name/description, unknown fields, whitespace | empty name, whitespace name, unknown field, happy path, audit log | ✅ Complete |
| `POST /api/v1/bundles/{id}/roles` | high | bundle id presence, project/role validity, duplicate | empty role_key, unknown field, happy path, DB error | ✅ Complete |
| `POST /api/v1/users/{id}/bundles` | high | non-empty bundle_id and user_id, unknown field rejection; **no explicit bundle-existence check** (FK violation surfaces as generic 500); **duplicate is transparent** via ON CONFLICT DO NOTHING | unknown field, empty bundle_id, idempotency (×2), audit attribution | ⚠️ Partial — existence pre-check and differentiated 404/409 not implemented |
| `POST /api/v1/rules/mapping` | critical | field presence, self-edge, cycle detection, unknown fields | missing fields, unknown field, self-edge, cycle detected, happy path + audit | ✅ Complete |
| `PUT /api/v1/rules/mapping/{id}` | medium | id presence, existence | not-found, happy path + audit, missing id | ✅ Complete |
| `POST /api/v1/users/{id}/grants` | critical | project/role validity, duration bounds, granted-by | expiry math (7d), zero-duration nil pointer, cache rebuild attribution | ✅ Complete |
| `POST /api/v1/requests` | critical | requester/project/role/justification required, duration bounds | persistence + audit, zero-duration nil pointer | ✅ Complete |
| `POST /api/v1/requests/{id}/decision` | critical | status enum, reviewer required on approve, idempotency guard | approve side effects, reject no-grant, already-approved 409, already-rejected 409, expiry from duration | ✅ Complete |
| `GET /api/v1/governance/summary` | high | response nil-safety | nil-safe [], pending count, unused bundle hint | ✅ Complete |
| `GET /api/v1/users/{id}/access` | critical | lineage nil-safety | nil-safe collections, source vs derived labeling, bundle reason kind, unknown user error, multi-hop derivation | ✅ Complete |
| `GET /api/v1/applications/{id}/simulate` | critical | user/app existence, claim formats | array/csv/space_delimited format outputs, unknown format fallback | ✅ Complete |
| `POST /api/action/inject` | critical | strict payload, cache miss, malformed cache safety | decode tests, empty-claim fallback, malformed cache, degraded modes | ✅ Complete |
| `POST /api/webhooks/zitadel` | critical | HMAC signature, freshness window, required fields | signature valid/invalid, stale timestamp, payload validation | ✅ Complete |
| internal Backend → Sync intent contract | high | authenticated payload, command semantics | — | ⏳ Deferred to Phase 4 |

### 6.3 Zitadel mechanism matrix

| Function or feature | Primary mechanism | Why this mechanism fits | Best-practice notes |
| --- | --- | --- | --- |
| token claim injection for downstream apps | Actions v2 | Runs in Zitadel's token flow and is the right place for custom claim emission | keep payload minimal, deterministic, and format-tested |
| userinfo/token-response claim shaping | Actions v2 | Same source-of-truth claim path and same compatibility boundary | never reimplement this through an internal-only shortcut |
| automated onboarding or policy triggers initiated by Zitadel events | Actions v2, with backend webhook/event intake where needed | preserves Zitadel-native trigger semantics while letting MkAuth evaluate policy | document the exact event contract and validate all incoming fields |
| backend reads or writes to grants, roles, memberships, or project assignments in Zitadel | service user account | requires Management API access that Actions v2 is not meant to replace | scope to least privilege, store credentials only server-side, rotate keys |
| mapping-rule propagation back into Zitadel | service user account | this is a backend control-plane mutation against the source of truth | require audit logging and idempotent retry behavior |
| administrative reconciliation jobs against Zitadel | service user account | long-running or bulk management work belongs in the backend control plane | isolate scopes, rate-limit, and add rollback-aware logging |
| frontend-to-backend admin actions | Zitadel-issued user access token validated by backend, then backend uses service user account if needed | keeps secrets and Zitadel management access out of the UI while preserving the acting user's identity | frontend never talks directly to Zitadel management APIs; shared internal API keys are optional defense-in-depth only |
| backend-to-sync-service provisioning intents | internal MkAuth contract | this is not a Zitadel contract and should stay private to MkAuth | mutually authenticate services and validate every command payload |

### 6.2 Service and utility matrix

| Area | Required coverage | Status |
| --- | --- | --- |
| cycle detection | acyclic insert, direct cycle, indirect cycle, duplicate edges, disconnected graphs | ✅ Complete |
| governance summary | pending request inclusion, nil-safe [], cleanup hint for unused bundles | ✅ Complete |
| claim formatting | array, csv, space-delimited outputs; unknown format fallback | ✅ Complete |
| lineage assembly | source roles, derived roles, direct grant reasons, bundle reasons, multi-hop, nil-safe | ✅ Complete |
| proxy authorization | member visibility limits, self-scoped access, admin passthrough, requester injection | ⏳ Deferred to Phase 3 frontend OIDC integration |
| boundary isolation | Zitadel Actions v2 external-only; internal FE/BE/sync contracts explicit and independently validated | ✅ Documented; enforcement deferred to live OIDC integration |

## 7. Document Drift to Resolve

### 7.1 Access-governance drift
The spec describes service-originated requests and bundle-oriented mapping context, while the current implementation primarily uses project/role request payloads.

### 7.2 Service-catalog drift
The portal exists, but the automatic service-to-bundle or service-to-role abstraction is only partially integrated.

### 7.3 Zero-trust drift
The design states the backend distrusts the UI and validates admin permissions, but the current backend authorization boundary is still a shared internal API key.

### 7.4 Testing and hardening visibility drift
The roadmap and coverage docs mention future security work, but they do not yet make contract hardening and backend-first tests the immediate next milestone.

### 7.5 Boundary-definition drift
The repo and docs do not yet state clearly enough that Zitadel-facing compatibility is anchored to Actions v2, while internal service-to-service payloads may use self-defined structures under separate hardening rules.

## 8. Documentation Update Rule
Any future work that changes request payloads, response shapes, validation rules, authorization assumptions, or database invariants must update all of the following in the same change:
* relevant OpenSpec capability specs
* the active design document if architecture assumptions moved
* the roadmap when milestone priority changes
* the feature coverage matrix when implementation reality changes

## 9. Phase 2 and Phase 3 Complete — Next Steps

Phase 2 contract hardening is complete. 82 backend tests cover all critical mutation endpoints, service logic, and claim paths. All listed DB constraints are in place. The injectable-dependency pattern is fully established for handler and service layers.

Phase 3 frontend OIDC integration is complete. The UI now performs a PKCE authorization code flow against Zitadel (`ui/src/lib/oidc.ts`, `ui/src/app/auth/zitadel/route.ts`, `ui/src/app/auth/callback/route.ts`). Zitadel-issued access tokens are stored in the session and forwarded to the backend on every API call. The shared API key is no longer the primary authorization proof for admin operations when `ZITADEL_DOMAIN` is set.

The immediate next step is the **Zitadel Management Client**: replace the stub `MgmtClient` in `backend/internal/zitadel/client.go` with an actual M2M Management API client, enabling live role CRUD and grant reconciliation against Zitadel.
