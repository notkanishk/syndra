# MkAuth OpenSpec Index

Navigation map for the specification surface. All paths relative to `openspec/`.

## Architecture Reference (Living Documents)

| Document | Purpose |
|----------|---------|
| [Core Design](changes/mkauth-core-architecture/design.md) | Architecture, planes, philosophy, Zitadel interaction matrix, IdP chain |
| [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) | Phase timeline with status, cross-references, and implementing changes |
| [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md) | Planned vs integrated reality matrix |

## Capability Specs

Each capability has one canonical spec. The "Origin" column shows provenance.

| Capability | Canonical Spec | Origin |
|-----------|---------------|--------|
| Role Management | [spec](changes/mkauth-core-architecture/specs/role-management/spec.md) | core-architecture + advanced-role-crud |
| User Management | [spec](changes/mkauth-core-architecture/specs/user-management/spec.md) | core-architecture |
| Access Governance | [spec](changes/mkauth-core-architecture/specs/access-governance/spec.md) | core-architecture + contract-hardening |
| Application Claims | [spec](changes/mkauth-core-architecture/specs/application-claims/spec.md) | core-architecture + backend-onboarding (merged) |
| Service Catalog | [spec](changes/mkauth-core-architecture/specs/service-catalog/spec.md) | core-architecture |
| Automation Policies | [spec](changes/mkauth-core-architecture/specs/automation-policies/spec.md) | core-architecture + backend-onboarding (merged) |
| LDAP Sync | [spec](changes/mkauth-core-architecture/specs/ldap-sync/spec.md) | core-architecture |
| Provisioning | [spec](changes/mkauth-core-architecture/specs/provisioning/spec.md) | core-architecture |
| Demo Catalog | [spec](changes/mkauth-core-architecture/specs/demo-catalog/spec.md) | core-architecture |
| Topology Graph | [spec](changes/mkauth-core-architecture/specs/topology-graph/spec.md) | core-architecture |
| Operational Readiness | [spec](changes/mkauth-core-architecture/specs/operational-readiness/spec.md) | core-architecture (Phase 5) |
| Contract Quality | [spec](changes/contract-hardening-and-test-foundation/specs/contract-quality/spec.md) | contract-hardening + backend-onboarding (merged) |
| Backend API Testing | [spec](changes/contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md) | contract-hardening + backend-onboarding (merged) |
| Production Security | [spec](changes/backend-owned-onboarding-and-security-boundary/specs/production-security-boundary/spec.md) | backend-onboarding |

## Change Log

Every change directory follows a standard structure: `proposal.md`, `design.md`, `tasks.md`, `IMPLEMENTATION.md`. Some also have `specs/`.

| Change | Phase | Status | Links |
|--------|-------|--------|-------|
| [Core Architecture](changes/mkauth-core-architecture/) | 1 | Complete | [proposal](changes/mkauth-core-architecture/proposal.md) / [design](changes/mkauth-core-architecture/design.md) / [tasks](changes/mkauth-core-architecture/tasks.md) / [impl](changes/mkauth-core-architecture/IMPLEMENTATION.md) |
| [Bun Native Migration](changes/bun-native-migration/) | 1 | Archived | [proposal](changes/bun-native-migration/proposal.md) / [design](changes/bun-native-migration/design.md) / [tasks](changes/bun-native-migration/tasks.md) / [impl](changes/bun-native-migration/IMPLEMENTATION.md) |
| [Contract Hardening](changes/contract-hardening-and-test-foundation/) | 2 | Complete | [proposal](changes/contract-hardening-and-test-foundation/proposal.md) / [design](changes/contract-hardening-and-test-foundation/design.md) / [tasks](changes/contract-hardening-and-test-foundation/tasks.md) / [impl](changes/contract-hardening-and-test-foundation/IMPLEMENTATION.md) |
| [Backend Onboarding & Security](changes/backend-owned-onboarding-and-security-boundary/) | 3 | Complete | [proposal](changes/backend-owned-onboarding-and-security-boundary/proposal.md) / [design](changes/backend-owned-onboarding-and-security-boundary/design.md) / [tasks](changes/backend-owned-onboarding-and-security-boundary/tasks.md) / [impl](changes/backend-owned-onboarding-and-security-boundary/IMPLEMENTATION.md) |
| [Codebase Audit](changes/codebase-audit-and-hardening/) | 3 | Complete | [proposal](changes/codebase-audit-and-hardening/proposal.md) / [design](changes/codebase-audit-and-hardening/design.md) / [tasks](changes/codebase-audit-and-hardening/tasks.md) / [impl](changes/codebase-audit-and-hardening/IMPLEMENTATION.md) |
| [Zitadel Management Client](changes/zitadel-management-client/) | 3 | Complete | [proposal](changes/zitadel-management-client/proposal.md) / [design](changes/zitadel-management-client/design.md) / [tasks](changes/zitadel-management-client/tasks.md) / [impl](changes/zitadel-management-client/IMPLEMENTATION.md) |
| [Live Webhook Listener](changes/live-webhook-listener/) | 3 | Complete | [proposal](changes/live-webhook-listener/proposal.md) / [design](changes/live-webhook-listener/design.md) / [tasks](changes/live-webhook-listener/tasks.md) / [impl](changes/live-webhook-listener/IMPLEMENTATION.md) |
| [Advanced Role CRUD](changes/advanced-role-crud/) | 3 | Complete | [proposal](changes/advanced-role-crud/proposal.md) / [design](changes/advanced-role-crud/design.md) / [tasks](changes/advanced-role-crud/tasks.md) / [impl](changes/advanced-role-crud/IMPLEMENTATION.md) |
| [Provisioning Intents](changes/provisioning-intents/) | 4 | Complete | [proposal](changes/provisioning-intents/proposal.md) / [design](changes/provisioning-intents/design.md) / [tasks](changes/provisioning-intents/tasks.md) / [impl](changes/provisioning-intents/IMPLEMENTATION.md) |
| [Shadow Password Vault](changes/shadow-password-vault/) | 4 | Complete | [proposal](changes/shadow-password-vault/proposal.md) / [design](changes/shadow-password-vault/design.md) / [tasks](changes/shadow-password-vault/tasks.md) / [impl](changes/shadow-password-vault/IMPLEMENTATION.md) |
| [Sync Service](changes/sync-service/) | 4 | Complete | [proposal](changes/sync-service/proposal.md) / [design](changes/sync-service/design.md) / [tasks](changes/sync-service/tasks.md) / [impl](changes/sync-service/IMPLEMENTATION.md) |

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
