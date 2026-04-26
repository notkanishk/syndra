# MkAuth OpenSpec Index

> **Looking for something specific?**
> - What's built vs not? → [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md)
> - What's the architecture? → [Core Design](changes/mkauth-core-architecture/design.md)
> - What's next? → [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) (Phase 5-6)
> - How was X decided? → Change Log below, then the relevant proposal

## Capability Specs

Each capability has one canonical spec. **Status** lets you skip specs that aren't relevant to your task.

| Capability | Status | Spec | Origin |
|-----------|--------|------|--------|
| Role Management | Integrated, per-role bundle/rule rollups in project view | [spec](changes/mkauth-core-architecture/specs/role-management/spec.md) | core-architecture + advanced-role-crud + dashboard-ux-elevation |
| User Management | Integrated, live Zitadel + Access Lineage chain + bundle preview + friendly expiry picker | [spec](changes/mkauth-core-architecture/specs/user-management/spec.md) | core-architecture + live-zitadel-data-source + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Access Governance | Integrated, urgency-tone expiring grants + audit timeline grouping/filters (bulk ops deferred P5) | [spec](changes/mkauth-core-architecture/specs/access-governance/spec.md) | core-architecture + contract-hardening + grant-expiration-scheduler + dashboard-ux-elevation |
| Application Claims | Integrated, Token Simulator copy/highlight/compare + reviewer notes in history | [spec](changes/mkauth-core-architecture/specs/application-claims/spec.md) | core-architecture + backend-onboarding + actions-v2-deployment + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Service Catalog | Integrated, inline modal Request Access flow (service-to-bundle mapping deferred P5) | [spec](changes/mkauth-core-architecture/specs/service-catalog/spec.md) | core-architecture + live-zitadel-data-source + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Automation Policies | Integrated, mapping-rule live preview + cycle warning (welcome bundle config UI deferred P5) | [spec](changes/mkauth-core-architecture/specs/automation-policies/spec.md) | core-architecture + backend-onboarding + dashboard-ux-elevation |
| LDAP Sync | Partial (reconciliation deferred P5, password compat unresolved) | [spec](changes/mkauth-core-architecture/specs/ldap-sync/spec.md) | core-architecture |
| Provisioning | Partial (reconciliation, compensating revocations deferred P5) | [spec](changes/mkauth-core-architecture/specs/provisioning/spec.md) | core-architecture |
| Demo Catalog | Integrated (local-dev fallback; bypassed when live Zitadel is configured; UI MUST NOT serialize demo entities in production) | [spec](changes/mkauth-core-architecture/specs/demo-catalog/spec.md) | core-architecture + live-zitadel-data-source + live-only-production-ui |
| Topology Graph | Integrated, pan/zoom + node deeplinks | [spec](changes/mkauth-core-architecture/specs/topology-graph/spec.md) | core-architecture + dashboard-ux-elevation |
| Operational Readiness | Integrated, toast + ConfirmModal + ErrorBoundary + theme toggle + sidebar activity badges + system/mode (LXC observability deferred P5) | [spec](changes/mkauth-core-architecture/specs/operational-readiness/spec.md) | core-architecture + live-only-production-ui + dashboard-ux-elevation |
| Contract Quality | Integrated | [spec](changes/contract-hardening-and-test-foundation/specs/contract-quality/spec.md) | contract-hardening + backend-onboarding |
| Backend API Testing | Integrated | [spec](changes/contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md) | contract-hardening + backend-onboarding |
| Production Security | Integrated | [spec](changes/backend-owned-onboarding-and-security-boundary/specs/production-security-boundary/spec.md) | backend-onboarding |

## Architecture Reference (Living Documents)

| Document | When to read |
|----------|-------------|
| [Core Design](changes/mkauth-core-architecture/design.md) | Understanding the system — planes, philosophy, Zitadel matrix, IdP chain |
| [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) | Understanding what's done and what's ahead — phase timeline with cross-refs |
| [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md) | Auditing spec vs reality — planned vs integrated matrix |

## Change Log

Every change directory has: `proposal.md` (why), `design.md` (how), `tasks.md` (breakdown), `IMPLEMENTATION.md` (what was built).

| Change | Phase | Status | Links |
|--------|-------|--------|-------|
| [Core Architecture](changes/mkauth-core-architecture/) | 1 | Complete | [proposal](changes/mkauth-core-architecture/proposal.md) / [design](changes/mkauth-core-architecture/design.md) / [impl](changes/mkauth-core-architecture/IMPLEMENTATION.md) |
| [Bun Migration](changes/bun-native-migration/) | 1 | Archived | [proposal](changes/bun-native-migration/proposal.md) / [impl](changes/bun-native-migration/IMPLEMENTATION.md) |
| [Contract Hardening](changes/contract-hardening-and-test-foundation/) | 2 | Complete | [proposal](changes/contract-hardening-and-test-foundation/proposal.md) / [design](changes/contract-hardening-and-test-foundation/design.md) / [impl](changes/contract-hardening-and-test-foundation/IMPLEMENTATION.md) |
| [Backend Onboarding & Security](changes/backend-owned-onboarding-and-security-boundary/) | 3 | Complete | [proposal](changes/backend-owned-onboarding-and-security-boundary/proposal.md) / [design](changes/backend-owned-onboarding-and-security-boundary/design.md) / [impl](changes/backend-owned-onboarding-and-security-boundary/IMPLEMENTATION.md) |
| [Codebase Audit](changes/codebase-audit-and-hardening/) | 3 | Complete | [proposal](changes/codebase-audit-and-hardening/proposal.md) / [design](changes/codebase-audit-and-hardening/design.md) / [impl](changes/codebase-audit-and-hardening/IMPLEMENTATION.md) |
| [Zitadel Management Client](changes/zitadel-management-client/) | 3 | Complete | [proposal](changes/zitadel-management-client/proposal.md) / [design](changes/zitadel-management-client/design.md) / [impl](changes/zitadel-management-client/IMPLEMENTATION.md) |
| [Live Webhook Listener](changes/live-webhook-listener/) | 3 | Complete | [proposal](changes/live-webhook-listener/proposal.md) / [design](changes/live-webhook-listener/design.md) / [impl](changes/live-webhook-listener/IMPLEMENTATION.md) |
| [Advanced Role CRUD](changes/advanced-role-crud/) | 3 | Complete | [proposal](changes/advanced-role-crud/proposal.md) / [design](changes/advanced-role-crud/design.md) / [impl](changes/advanced-role-crud/IMPLEMENTATION.md) |
| [Provisioning Intents](changes/provisioning-intents/) | 4 | Complete | [proposal](changes/provisioning-intents/proposal.md) / [design](changes/provisioning-intents/design.md) / [impl](changes/provisioning-intents/IMPLEMENTATION.md) |
| [Shadow Password Vault](changes/shadow-password-vault/) | 4 | Complete | [proposal](changes/shadow-password-vault/proposal.md) / [design](changes/shadow-password-vault/design.md) / [impl](changes/shadow-password-vault/IMPLEMENTATION.md) |
| [Sync Service](changes/sync-service/) | 4 | Complete | [proposal](changes/sync-service/proposal.md) / [design](changes/sync-service/design.md) / [impl](changes/sync-service/IMPLEMENTATION.md) |
| [Zitadel Diagnostic UI](changes/zitadel-diagnostic-ui/) | 5 | Complete | [proposal](changes/zitadel-diagnostic-ui/proposal.md) / [design](changes/zitadel-diagnostic-ui/design.md) / [impl](changes/zitadel-diagnostic-ui/IMPLEMENTATION.md) |
| [Zitadel Actions v2 Deployment](changes/zitadel-actions-v2-deployment/) | 5 | Complete | [proposal](changes/zitadel-actions-v2-deployment/proposal.md) / [design](changes/zitadel-actions-v2-deployment/design.md) / [impl](changes/zitadel-actions-v2-deployment/IMPLEMENTATION.md) / [deploy](changes/zitadel-actions-v2-deployment/DEPLOY.md) |
| [Grant Expiration Scheduler](changes/grant-expiration-scheduler/) | 5 | Complete | [proposal](changes/grant-expiration-scheduler/proposal.md) / [design](changes/grant-expiration-scheduler/design.md) / [tasks](changes/grant-expiration-scheduler/tasks.md) |
| [Live Zitadel Data Source](changes/live-zitadel-data-source/) | 5 | Complete | [proposal](changes/live-zitadel-data-source/proposal.md) / [design](changes/live-zitadel-data-source/design.md) / [tasks](changes/live-zitadel-data-source/tasks.md) |
| [Live-Directory Identity Completeness](changes/live-directory-identity-completeness/) | 5 | Complete | [proposal](changes/live-directory-identity-completeness/proposal.md) / [design](changes/live-directory-identity-completeness/design.md) / [tasks](changes/live-directory-identity-completeness/tasks.md) |
| [Live-Only Production UI](changes/live-only-production-ui/) | 5 | Complete | [proposal](changes/live-only-production-ui/proposal.md) / [design](changes/live-only-production-ui/design.md) / [tasks](changes/live-only-production-ui/tasks.md) |
| [Dashboard UX Elevation](changes/dashboard-ux-elevation/) | 5 | Complete | [proposal](changes/dashboard-ux-elevation/proposal.md) / [design](changes/dashboard-ux-elevation/design.md) / [tasks](changes/dashboard-ux-elevation/tasks.md) |

## Roadmap Phase -> Change Mapping

| Phase | Status | Changes |
|-------|--------|---------|
| 1: UI Baseline | Complete | core-architecture, bun-migration |
| 2: Contract Hardening | Complete | contract-hardening |
| 3: Security Boundary | Complete | backend-onboarding, codebase-audit, zitadel-mgmt-client, webhook-listener, role-crud |
| 4: Infrastructure Bridge | In Progress | provisioning-intents, shadow-vault, sync-service |
| 5: Automation & Governance | In Progress | zitadel-actions-v2-deployment, grant-expiration-scheduler, live-zitadel-data-source, live-directory-identity-completeness, live-only-production-ui, dashboard-ux-elevation; [remaining items](changes/mkauth-core-architecture/ROADMAP.md) |
| 6: IdP Lifecycle | Not Started | [see Roadmap](changes/mkauth-core-architecture/ROADMAP.md) |
