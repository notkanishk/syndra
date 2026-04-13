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

## 3. Current Risk Summary

### 3.1 Transport validation risk
Current handlers decode JSON directly into request structs and mostly check only for missing required strings. This leaves gaps such as:
* unknown field acceptance
* weak enum handling
* unbounded strings
* negative or unrealistic durations
* inconsistent reviewer and resolution semantics

### 3.2 Type ownership risk
The repo currently duplicates similar interfaces across many UI routes and components while the backend uses broad string fields and generic maps for some important payloads. This increases drift risk and weakens compile-time guarantees.

### 3.3 Persistence invariant risk
The schema uses `NOT NULL` and uniqueness in useful places, but it does not yet fully encode domain invariants such as valid statuses, positive durations, claim format enums, version bounds, or resolved-state consistency.

### 3.4 Authorization boundary risk
The current frontend proxy enforces session-role behavior for demo flows, but the backend still primarily trusts a shared API key. That is acceptable for local development but does not satisfy the design's zero-trust intent.

### 3.5 Regression risk
The codebase currently lacks app-owned backend unit tests and frontend UI tests, making it easy for critical behavior to change silently.

### 3.6 External contract drift risk
Without an explicit rule that Zitadel-facing communication stays aligned to Actions v2, internal shortcuts can accidentally leak into the source-of-truth boundary and create brittle or insecure integration assumptions.

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

| Endpoint | Risk | Required validation focus | Required test focus |
| --- | --- | --- | --- |
| `POST /api/v1/bundles` | medium | name/description constraints, unknown fields, duplicate semantics | handler success/failure cases, duplicate bundle behavior |
| `POST /api/v1/bundles/{id}/roles` | high | bundle id presence, project/role validity, duplicate role mapping | validation tests, repository conflict tests, audit coverage |
| `POST /api/v1/users/{id}/bundles` | high | self vs admin rules, bundle existence, duplicate assignment | handler tests, auth tests, idempotency behavior |
| `POST /api/v1/rules/mapping` | critical | strict field presence, normalized ids, self-edge policy, cycle detection | unit tests for cycle detection, handler conflict tests, invalid payload tests |
| `PUT /api/v1/rules/mapping/{id}` | medium | valid id, existence, version semantics | not-found tests, version increment tests |
| `POST /api/v1/users/{id}/grants` | critical | project/role validity, duration bounds, reason rules, granted-by semantics | expiry math tests, invalid duration tests, audit and cache rebuild expectations |
| `POST /api/v1/requests` | critical | requester identity, target validity, justification bounds, duration bounds | member/admin path tests, invalid payload tests, persistence tests |
| `POST /api/v1/requests/{id}/decision` | critical | allowed status transitions, reviewer requirements, resolution invariants | approve/reject tests, duplicate resolution tests, direct-grant side effect tests |
| `GET /api/v1/governance/summary` | high | response stability and nil-safe collections | snapshot/shape tests, expiring grant filtering |
| `GET /api/v1/users/{id}/access` | critical | lineage completeness, nil-safe response sections | service tests for source vs derived roles, direct grant inclusion |
| `GET /api/v1/applications/{id}/simulate` | critical | user/app existence, allowed claim formats, deterministic output | claim-format tests, regression tests for array/csv/space-delimited shaping |
| `POST /api/action/inject` | critical | strict action payload, cache miss handling, malformed cache safety | decode tests, empty-claim fallback tests, malformed cache payload tests |
| `POST /api/webhooks/zitadel` | critical | payload authenticity model, required fields, safe invalidation behavior | invalid payload tests, orchestrator failure tests, project-id branch coverage |
| internal Backend -> Sync intent contract | high | authenticated internal payload shape, explicit command semantics, no Zitadel-specific leakage into internal types | validator tests, auth tests, compatibility tests for boundary isolation |

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

| Area | Required coverage |
| --- | --- |
| cycle detection | acyclic insert, direct cycle, indirect cycle, duplicate edges, disconnected graphs |
| governance summary | pending request inclusion, expiring window behavior, cleanup hint stability |
| claim formatting | array, csv, and space-delimited outputs; invalid format rejection |
| lineage assembly | source roles, derived roles, direct grant reasons, bundle reasons, cleanup hints |
| proxy authorization | member visibility limits, self-scoped access, admin passthrough, requester injection |
| boundary isolation | Zitadel Actions v2 assumptions remain external-only, while internal FE/BE/sync contracts stay explicit and independently validated |

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

## 9. Immediate Next Step
Before MkAuth expands its live Zitadel and provisioning scope, the project should complete a dedicated hardening wave that formalizes contracts, adds strict validation, and lands backend-first regression coverage.
