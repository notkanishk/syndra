# MkAuth OpenSpec Index

Navigation map for the specification surface. All paths relative to `openspec/`.

## Architecture Reference (Living Documents)

| Document | Purpose |
|----------|---------|
| [Core Design](changes/mkauth-core-architecture/design.md) | Architecture, planes, philosophy, Zitadel interaction matrix |
| [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) | Phase timeline with status and implementing changes |
| [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md) | Planned vs integrated reality matrix |

## Capability Specs

| Capability | Primary Spec | Modified By |
|-----------|-------------|-------------|
| Role Management | [spec](changes/mkauth-core-architecture/specs/role-management/spec.md) | [advanced-role-crud](changes/advanced-role-crud/) |
| User Management | [spec](changes/mkauth-core-architecture/specs/user-management/spec.md) | -- |
| Access Governance | [spec](changes/mkauth-core-architecture/specs/access-governance/spec.md) | [contract-hardening](changes/contract-hardening-and-test-foundation/), [backend-onboarding](changes/backend-owned-onboarding-and-security-boundary/) |
| Application Claims | [spec](changes/mkauth-core-architecture/specs/application-claims/spec.md) | [contract-hardening](changes/contract-hardening-and-test-foundation/), [backend-onboarding](changes/backend-owned-onboarding-and-security-boundary/) |
| Service Catalog | [spec](changes/mkauth-core-architecture/specs/service-catalog/spec.md) | -- |
| Automation Policies | [spec](changes/mkauth-core-architecture/specs/automation-policies/spec.md) | [backend-onboarding](changes/backend-owned-onboarding-and-security-boundary/) |
| LDAP Sync | [spec](changes/mkauth-core-architecture/specs/ldap-sync/spec.md) | [sync-service](changes/sync-service/) |
| Provisioning | [spec](changes/mkauth-core-architecture/specs/provisioning/spec.md) | [provisioning-intents](changes/provisioning-intents/) |
| Demo Catalog | [spec](changes/mkauth-core-architecture/specs/demo-catalog/spec.md) | -- |
| Topology Graph | [spec](changes/mkauth-core-architecture/specs/topology-graph/spec.md) | -- |
| Contract Quality | [spec](changes/contract-hardening-and-test-foundation/specs/contract-quality/spec.md) | [backend-onboarding](changes/backend-owned-onboarding-and-security-boundary/) |
| Backend API Testing | [spec](changes/contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md) | [backend-onboarding](changes/backend-owned-onboarding-and-security-boundary/) |
| Production Security | [spec](changes/backend-owned-onboarding-and-security-boundary/specs/production-security-boundary/spec.md) | -- |

## Change Log

| Change | Phase | Status | Links |
|--------|-------|--------|-------|
| [Core Architecture](changes/mkauth-core-architecture/) | 1 | Complete | [proposal](changes/mkauth-core-architecture/proposal.md) / [design](changes/mkauth-core-architecture/design.md) / [tasks](changes/mkauth-core-architecture/tasks.md) |
| [Bun Native Migration](changes/bun-native-migration/) | 1 | Stale | [proposal](changes/bun-native-migration/proposal.md) / [design](changes/bun-native-migration/design.md) / [tasks](changes/bun-native-migration/tasks.md) |
| [Contract Hardening](changes/contract-hardening-and-test-foundation/) | 2 | Complete | [proposal](changes/contract-hardening-and-test-foundation/proposal.md) / [design](changes/contract-hardening-and-test-foundation/design.md) / [tasks](changes/contract-hardening-and-test-foundation/tasks.md) |
| [Backend Onboarding & Security](changes/backend-owned-onboarding-and-security-boundary/) | 3 | Complete | [proposal](changes/backend-owned-onboarding-and-security-boundary/proposal.md) / [design](changes/backend-owned-onboarding-and-security-boundary/design.md) / [tasks](changes/backend-owned-onboarding-and-security-boundary/tasks.md) |
| [Codebase Audit](changes/codebase-audit-and-hardening/) | 3 | Complete | [proposal](changes/codebase-audit-and-hardening/proposal.md) / [design](changes/codebase-audit-and-hardening/design.md) / [tasks](changes/codebase-audit-and-hardening/tasks.md) |
| [Zitadel Management Client](changes/zitadel-management-client/) | 3 | Complete | [proposal](changes/zitadel-management-client/proposal.md) / [design](changes/zitadel-management-client/design.md) / [tasks](changes/zitadel-management-client/tasks.md) |
| [Live Webhook Listener](changes/live-webhook-listener/) | 3 | Complete | [proposal](changes/live-webhook-listener/proposal.md) / [design](changes/live-webhook-listener/design.md) / [tasks](changes/live-webhook-listener/tasks.md) |
| [Advanced Role CRUD](changes/advanced-role-crud/) | 3 | Complete | [proposal](changes/advanced-role-crud/proposal.md) / [design](changes/advanced-role-crud/design.md) / [tasks](changes/advanced-role-crud/tasks.md) |
| [Provisioning Intents](changes/provisioning-intents/) | 4 | Complete | [proposal](changes/provisioning-intents/proposal.md) / [design](changes/provisioning-intents/design.md) / [tasks](changes/provisioning-intents/tasks.md) |
| [Shadow Password Vault](changes/shadow-password-vault/) | 4 | Complete | [proposal](changes/shadow-password-vault/proposal.md) / [design](changes/shadow-password-vault/design.md) / [tasks](changes/shadow-password-vault/tasks.md) |
| [Sync Service](changes/sync-service/) | 4 | Complete | [proposal](changes/sync-service/proposal.md) / [design](changes/sync-service/design.md) / [tasks](changes/sync-service/tasks.md) |

## Roadmap Phase -> Change Mapping

| Phase | Implementing Changes |
|-------|---------------------|
| 1: UI Baseline | core-architecture, bun-native-migration |
| 2: Contract Hardening | contract-hardening-and-test-foundation |
| 3: Security Boundary | backend-owned-onboarding, codebase-audit, zitadel-management-client, live-webhook-listener, advanced-role-crud |
| 4: Infrastructure Bridge | provisioning-intents, shadow-password-vault, sync-service |
| 5: Automation & Governance | (not started -- see [Roadmap](changes/mkauth-core-architecture/ROADMAP.md)) |
| 6: IdP Lifecycle | (not started -- see [Roadmap](changes/mkauth-core-architecture/ROADMAP.md)) |

## Reading Order for New Contributors

1. [Core Design](changes/mkauth-core-architecture/design.md) -- understand the three planes and philosophy
2. [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) -- understand what's done and what's ahead
3. [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md) -- understand spec vs reality
4. Capability specs relevant to your work (table above)
5. Implementing change proposals for context on decisions made
