## Why

Syndra's current backend and UI already prove the product shape, but the contract layer underneath them is still too permissive for a system that will ultimately govern identity, authorization, access lineage, and downstream claim issuance. The current codebase relies on a mix of descriptive structs, handwritten handler checks, UI-local TypeScript interfaces, and partial database invariants. That leaves avoidable room for malformed input, undocumented edge cases, contract drift between backend and frontend, and regressions that are especially dangerous in security-sensitive flows.

The next immediate step must therefore be hardening and formalizing the application's types, schemas, validation rules, and test coverage before more feature work expands the surface area.

## What Changes

* Defines a dedicated quality contract for backend and frontend request/response schemas, domain types, and persistence invariants.
* Establishes backend-first validation and testing standards for all mission-critical endpoints and service logic.
* Documents the current contract drift between specs and implementation, especially around service-catalog access requests, authorization boundaries, and the shift from shared internal API-key trust toward Zitadel-issued user access tokens for privileged frontend requests.
* Makes Zitadel Actions v2 compatibility the required external contract boundary for source-of-truth communication, while allowing hardened purpose-built contracts for internal service-to-service communication.
* Updates the roadmap and existing architecture docs so hardening and testing are explicitly the immediate next step.

## Capabilities

### New Capabilities
* `contract-quality`: Standards for strict schemas, purpose-built domain types, database invariants, and documentation synchronization.
* `backend-api-testing`: Endpoint-by-endpoint validation and unit test expectations, prioritizing backend mission-critical flows first and frontend UI contracts second.

### Modified Capabilities
* `access-governance`: Clarifies the difference between currently implemented project/role requests and the intended service-to-bundle abstraction.
* `service-catalog`: Clarifies that service requests are still only partially integrated until they are backed by hardened contracts and automated mapping.
* `application-claims`: Elevates schema guarantees and regression coverage around claim shaping and action payloads.
* `provisioning`: Clarifies that internal Backend-to-Sync contracts may be self-defined, but must remain isolated from and compatible with the Zitadel Actions v2 source-of-truth boundary.

## Impact

* Affects backend handlers, service-layer validation, database migrations, cache/data-plane claim contracts, provisioning intents, and API error semantics.
* Affects frontend type ownership, proxy authorization boundaries, request forms, and UI-facing API assumptions.
* Establishes the acceptance bar future implementation work must meet before being treated as production-grade.
