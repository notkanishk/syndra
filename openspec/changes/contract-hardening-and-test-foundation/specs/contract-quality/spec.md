> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Strict API request contracts
The system MUST reject malformed or ambiguous mutation payloads at the backend boundary before business logic executes.

#### Scenario: Unknown request field rejected
- **WHEN** a client submits a mutation payload containing fields that are not part of the documented API contract
- **THEN** the backend MUST reject the request with a validation error
- **AND** the response MUST use a stable error code

#### Scenario: Invalid enum rejected
- **WHEN** a client submits a bounded field such as a request status or claim format with an unsupported value
- **THEN** the backend MUST reject the request rather than coercing or ignoring the value

### Requirement: Purpose-built type boundaries
The system MUST maintain distinct transport, domain, persistence, and UI model boundaries for security-critical data.

#### Scenario: Transport and persistence models diverge safely
- **WHEN** a new persistence field is added for internal bookkeeping
- **THEN** that field MUST NOT automatically become part of the public API response contract unless explicitly modeled and documented

### Requirement: Persistence invariants mirror domain rules
The system MUST enforce critical domain rules in the database as well as in application validation.

#### Scenario: Invalid resolved request state blocked
- **WHEN** a request is stored with a terminal status but no reviewer or resolution timestamp
- **THEN** the persistence layer MUST reject the invalid state

### Requirement: Stable error responses
The system MUST expose predictable validation and conflict responses for documented contract failures.

#### Scenario: Field validation failure
- **WHEN** a client submits an invalid payload
- **THEN** the backend MUST return a stable machine-readable error code
- **AND** the response MUST provide enough detail for the caller to identify the invalid field or rule

### Requirement: Documentation stays synchronized with contracts
The system MUST update relevant OpenSpec docs whenever contract semantics change.

#### Scenario: Request contract changes
- **WHEN** a request or response schema changes for a documented capability
- **THEN** the relevant capability spec, roadmap references, and implementation coverage notes MUST be updated in the same change

### Requirement: Zitadel source-of-truth compatibility is preserved
The system MUST treat Zitadel Actions v2 as the canonical compatibility boundary for source-of-truth claim and event interactions.

#### Scenario: External contract review
- **WHEN** a Zitadel-facing payload, flow, or integration assumption changes
- **THEN** the change MUST preserve Actions v2 compatibility
- **AND** the security and compatibility impact on the source-of-truth boundary MUST be documented

### Requirement: Internal contracts remain isolated and hardened
The system MAY use self-defined structures for communication between the frontend UI, backend, and sync service, but those contracts MUST be explicit, authenticated, validated, and isolated from the Zitadel-facing boundary.

#### Scenario: Internal sync payload introduced
- **WHEN** MkAuth defines a Backend-to-Sync payload for provisioning work
- **THEN** that payload MAY use a purpose-built internal schema
- **AND** it MUST NOT redefine or loosen the Zitadel Actions v2 compatibility requirements used at the external boundary

#### Scenario: High-risk internal credential payload review
- **WHEN** an internal Backend-to-Sync or credential-bridge payload contains infrastructure secrets such as Samba/LLDAP password material
- **THEN** that payload MUST be treated as a high-risk internal contract
- **AND** it MUST receive stricter validation, authentication, auditability, and design review than ordinary internal payloads

### Requirement: Production orchestration trust boundary is explicit
The system MUST treat production authorization and orchestration edges as explicit contract boundaries rather than as deployment-time assumptions.

#### Scenario: Privileged production path review
- **WHEN** a backend-owned orchestration path can grant, revoke, onboard, or emit provisioning work in production
- **THEN** the contract for that path MUST define authentication, authorization, idempotency, observability, and failure behavior
- **AND** the path MUST NOT rely on undocumented trust in frontend or trigger-origin assumptions

### Requirement: Service account usage is narrowly scoped
The system MUST use a dedicated Zitadel service user account only for backend-owned management operations that require Management API access.

#### Scenario: Admin action requires Zitadel mutation
- **WHEN** an admin-triggered MkAuth workflow needs to create, update, or revoke state in Zitadel
- **THEN** the backend MAY use a dedicated service user account to call the Zitadel Management API
- **AND** that credential MUST remain server-side, least-privileged, and independently auditable
- **AND** the frontend MUST NOT use it directly

### Requirement: User identity is preserved to the backend boundary
The system MUST use a Zitadel-issued user access token as the primary identity proof for privileged frontend-to-backend requests in production.

#### Scenario: Privileged admin request reaches backend
- **WHEN** a privileged admin action is submitted from the frontend
- **THEN** the request MUST carry a Zitadel-issued user access token
- **AND** the backend MUST validate that token and authorize the acting user directly
- **AND** a shared internal API key alone MUST NOT be treated as sufficient production authorization
