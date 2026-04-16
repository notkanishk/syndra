> **Spec consolidation note:** The 4 modified capability specs originally in this change have been merged into their canonical locations. See [automation-policies](../mkauth-core-architecture/specs/automation-policies/spec.md), [application-claims](../mkauth-core-architecture/specs/application-claims/spec.md), [contract-quality](../contract-hardening-and-test-foundation/specs/contract-quality/spec.md), [backend-api-testing](../contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md). Implementation details are in [IMPLEMENTATION.md](IMPLEMENTATION.md). The only spec unique to this change is [production-security-boundary](specs/production-security-boundary/spec.md).

## Why

MkAuth's architecture now clearly says two important things:

* onboarding mutations such as Welcome Bundle assignment should be backend-owned, auditable, and retry-safe rather than split across Zitadel-hosted logic and backend control-plane logic
* production rollout must be gated on closing the trust boundary around per-admin authorization, action-injection security, webhook authenticity, and other high-risk orchestration edges

Those decisions are reflected in the architecture and roadmap docs, but they do not yet exist as a dedicated implementation change. Without that follow-up change, the project risks drifting back toward mixed mutation ownership, ambiguous degraded behavior in the data plane, and shipping live orchestration before the security boundary is actually closed.

## What Changes

* Defines a backend-owned onboarding path for Welcome Bundles and similar event-driven assignment policies.
* Establishes a dedicated production security-boundary milestone covering backend user-token authorization, action-injection perimeter hardening, webhook authenticity validation, and operator-visible degraded behavior.
* Clarifies the responsibility split between Zitadel-compatible trigger intake and backend-owned business mutations.
* Adds explicit tasking and coverage expectations for safe rollout of live Zitadel orchestration.

## Capabilities

### New Capabilities
* `production-security-boundary`: Requirements for the minimum trust-boundary controls MkAuth must satisfy before live orchestration is treated as production-ready.

### Modified Capabilities
* `automation-policies`: Welcome-bundle automation becomes explicitly backend-owned after validated event intake.
* `application-claims`: Data-plane claim injection gains explicit degraded-mode and perimeter expectations for production operation.
* `contract-quality`: Extends the contract hardening posture to cover production trust-boundary guarantees for privileged orchestration flows.
* `backend-api-testing`: Adds security-boundary coverage requirements for onboarding, action-injection, and webhook authenticity paths.

## Impact

* Affects backend token validation and authorization, Zitadel event intake, welcome-bundle assignment flow, action-injection behavior, webhook handling, audit expectations, and rollout sequencing.
* Tightens what “production-ready” means for live Zitadel and provisioning integration.
* Creates the implementation-ready doc baseline for the next backend-first milestone.
