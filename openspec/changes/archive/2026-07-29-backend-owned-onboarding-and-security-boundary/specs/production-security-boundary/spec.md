> **Status:** Integrated | [< Index](../../../../INDEX.md)

# Requirement: Production Trust Boundary

> Implementation details for all requirements below are in [IMPLEMENTATION.md](../../IMPLEMENTATION.md).

## ADDED Requirements

### Requirement: Production trust boundary gate
Syndra MUST satisfy explicit trust-boundary controls before live Zitadel-backed orchestration is treated as production-ready.

#### Scenario: Production rollout readiness review
- **WHEN** the project is evaluated for live orchestration readiness
- **THEN** the system MUST demonstrate backend user-token authorization, validated webhook authenticity, bounded and observable action-injection behavior, and documented degraded behavior for claim injection
- **AND** missing any of those controls MUST block production-ready classification

### Requirement: Backend authorization is authoritative for privileged mutations
The backend MUST be the final authorization authority for privileged administrative mutations.

#### Scenario: Admin mutation reaches backend
- **WHEN** a privileged grant, revoke, bundle assignment, or onboarding mutation is submitted
- **THEN** the request MUST carry a Zitadel-issued user access token
- **AND** the backend MUST validate that token, identify the acting admin, and evaluate their authorization before executing the mutation
- **AND** possession of a shared internal API key alone MUST NOT be treated as sufficient production authorization

### Requirement: Webhook authenticity is verified before orchestration
The system MUST verify webhook authenticity and freshness before allowing cache invalidation, onboarding triggers, or downstream mutation work to proceed.

#### Scenario: Unverified webhook received
- **WHEN** Syndra receives a structurally valid but unverified webhook payload
- **THEN** the system MUST reject it as non-authoritative for orchestration
- **AND** no downstream mutation or cache invalidation MUST occur

### Requirement: Action-injection perimeter is production-hardened
The claim-injection path MUST be bounded, observable, and operate with a documented security posture. The endpoint is intentionally unauthenticated because it is called by Zitadel Actions v2 during the token flow; security relies on network isolation (internal-only access) and deterministic degraded behavior rather than caller authentication.

#### Scenario: Action injection under degraded dependency conditions
- **WHEN** the claim path encounters a timeout, cache miss, malformed cache entry, or unreachable dependency
- **THEN** the system MUST apply the application's documented failure posture
- **AND** the degraded outcome MUST be observable to operators

### Requirement: High-risk orchestration failures are auditable
The system MUST leave an auditable trail for onboarding and other high-risk orchestration outcomes.

#### Scenario: Welcome-bundle assignment fails
- **WHEN** Syndra cannot complete a backend-owned onboarding mutation
- **THEN** the failed attempt MUST be visible through audit or operator-facing diagnostics
- **AND** the retry path MUST avoid duplicate grants
