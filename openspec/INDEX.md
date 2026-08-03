# MkAuth OpenSpec Index

> **Looking for something specific?**
> - What's built vs not? → [Feature Coverage](changes/mkauth-core-architecture/specs/feature-coverage.md)
> - What's the architecture? → [Core Design](changes/mkauth-core-architecture/design.md)
> - What's next? → **[NEXT.md](NEXT.md)** — every open gap in one place. Long-range framing: [Roadmap](changes/mkauth-core-architecture/ROADMAP.md) (Phase 5-6)
> - How was X decided? → Change Log below, then the relevant proposal

## Capability Specs

Each capability has one canonical spec. **Status** lets you skip specs that aren't relevant to your task.

| Capability | Status | Spec | Origin |
|-----------|--------|------|--------|
| Role Management | Integrated, cross-project index + role → members with access sources | [spec](changes/mkauth-core-architecture/specs/role-management/spec.md) | core-architecture + advanced-role-crud + dashboard-ux-elevation |
| User Management | Integrated, person detail + direct-grant removal + member surface | [spec](changes/mkauth-core-architecture/specs/user-management/spec.md) | core-architecture + live-zitadel-data-source + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Access Governance | Integrated, Basic/Advanced IA + indicators endpoint + source-specific removal (bulk ops deferred P5) | [spec](changes/mkauth-core-architecture/specs/access-governance/spec.md) | core-architecture + contract-hardening + grant-expiration-scheduler + dashboard-ux-elevation |
| Application Claims | Integrated, **operator-editable token shape applied to real tokens** (shared shaper, per-app overrides) | [spec](changes/mkauth-core-architecture/specs/application-claims/spec.md) | core-architecture + backend-onboarding + actions-v2-deployment + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Service Catalog | Integrated, inline modal Request Access flow (service-to-bundle mapping deferred P5) | [spec](changes/mkauth-core-architecture/specs/service-catalog/spec.md) | core-architecture + live-zitadel-data-source + live-directory-identity-completeness + live-only-production-ui + dashboard-ux-elevation |
| Automation Policies | Integrated, mapping-rule live preview + cycle warning (welcome bundle config UI deferred P5) | [spec](changes/mkauth-core-architecture/specs/automation-policies/spec.md) | core-architecture + backend-onboarding + dashboard-ux-elevation |
| LDAP Sync | Partial (reconciliation deferred P5, password compat unresolved) | [spec](changes/mkauth-core-architecture/specs/ldap-sync/spec.md) | core-architecture |
| Provisioning | Partial (reconciliation, compensating revocations deferred P5) | [spec](changes/mkauth-core-architecture/specs/provisioning/spec.md) | core-architecture |
| Demo Catalog | Integrated (local-dev fallback; bypassed when live Zitadel is configured; UI MUST NOT serialize demo entities in production) | [spec](changes/mkauth-core-architecture/specs/demo-catalog/spec.md) | core-architecture + live-zitadel-data-source + live-only-production-ui |
| Topology Graph | Integrated, pan/zoom + node deeplinks | [spec](changes/mkauth-core-architecture/specs/topology-graph/spec.md) | core-architecture + dashboard-ux-elevation |
| Operational Readiness | Integrated, toast + ConfirmModal + ErrorBoundary + theme toggle + sidebar activity badges + system/mode (LXC observability deferred P5) | [spec](changes/mkauth-core-architecture/specs/operational-readiness/spec.md) | core-architecture + live-only-production-ui + dashboard-ux-elevation |
| Contract Quality | Integrated | [spec](specs/contract-quality/spec.md) | contract-hardening + backend-onboarding |
| Backend API Testing | Integrated | [spec](specs/backend-api-testing/spec.md) | contract-hardening + backend-onboarding |
| Production Security | Integrated | [spec](specs/production-security-boundary/spec.md) | backend-onboarding |
| Lifecycle Event Propagation | Integrated | [spec](changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md) | zitadel-event-trigger-propagation |

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
| [Contract Hardening](changes/archive/2026-07-29-contract-hardening-and-test-foundation/) | 2 | Archived | [proposal](changes/archive/2026-07-29-contract-hardening-and-test-foundation/proposal.md) / [design](changes/archive/2026-07-29-contract-hardening-and-test-foundation/design.md) / [impl](changes/archive/2026-07-29-contract-hardening-and-test-foundation/IMPLEMENTATION.md) |
| [Backend Onboarding & Security](changes/archive/2026-07-29-backend-owned-onboarding-and-security-boundary/) | 3 | Archived | [proposal](changes/archive/2026-07-29-backend-owned-onboarding-and-security-boundary/proposal.md) / [design](changes/archive/2026-07-29-backend-owned-onboarding-and-security-boundary/design.md) / [impl](changes/archive/2026-07-29-backend-owned-onboarding-and-security-boundary/IMPLEMENTATION.md) |
| [Codebase Audit](changes/codebase-audit-and-hardening/) | 3 | Complete | [proposal](changes/codebase-audit-and-hardening/proposal.md) / [design](changes/codebase-audit-and-hardening/design.md) / [impl](changes/codebase-audit-and-hardening/IMPLEMENTATION.md) |
| [Zitadel Management Client](changes/zitadel-management-client/) | 3 | Complete | [proposal](changes/zitadel-management-client/proposal.md) / [design](changes/zitadel-management-client/design.md) / [impl](changes/zitadel-management-client/IMPLEMENTATION.md) |
| [Live Webhook Listener](changes/live-webhook-listener/) | 3 | Complete | [proposal](changes/live-webhook-listener/proposal.md) / [design](changes/live-webhook-listener/design.md) / [impl](changes/live-webhook-listener/IMPLEMENTATION.md) |
| [Advanced Role CRUD](changes/advanced-role-crud/) | 3 | Complete | [proposal](changes/advanced-role-crud/proposal.md) / [design](changes/advanced-role-crud/design.md) / [impl](changes/advanced-role-crud/IMPLEMENTATION.md) |
| [Provisioning Intents](changes/provisioning-intents/) | 4 | Complete | [proposal](changes/provisioning-intents/proposal.md) / [design](changes/provisioning-intents/design.md) / [impl](changes/provisioning-intents/IMPLEMENTATION.md) |
| [Shadow Password Vault](changes/shadow-password-vault/) | 4 | Complete | [proposal](changes/shadow-password-vault/proposal.md) / [design](changes/shadow-password-vault/design.md) / [impl](changes/shadow-password-vault/IMPLEMENTATION.md) |
| [Sync Service](changes/sync-service/) | 4 | Complete | [proposal](changes/sync-service/proposal.md) / [design](changes/sync-service/design.md) / [impl](changes/sync-service/IMPLEMENTATION.md) |
| [Zitadel Diagnostic UI](changes/archive/2026-07-29-zitadel-diagnostic-ui/) | 5 | Archived | [proposal](changes/archive/2026-07-29-zitadel-diagnostic-ui/proposal.md) / [design](changes/archive/2026-07-29-zitadel-diagnostic-ui/design.md) / [impl](changes/archive/2026-07-29-zitadel-diagnostic-ui/IMPLEMENTATION.md) |
| [Basic / Advanced IA](changes/basic-advanced-ia/) | 5 | Complete | [proposal](changes/basic-advanced-ia/proposal.md) / [design](changes/basic-advanced-ia/design.md) / [tasks](changes/basic-advanced-ia/tasks.md) |
| [IA Screen Completion](changes/ia-screen-completion/) | 5 | Complete | [proposal](changes/ia-screen-completion/proposal.md) / [design](changes/ia-screen-completion/design.md) / [tasks](changes/ia-screen-completion/tasks.md) |
| [Operator Runbook Surfaces](changes/operator-runbook-surfaces/) | 5 | Complete | [proposal](changes/operator-runbook-surfaces/proposal.md) / [tasks](changes/operator-runbook-surfaces/tasks.md) |
| [People Bulk & Dashboard Depth](changes/people-bulk-and-dashboard-depth/) | 5 | Complete (2 operator-gated) | [proposal](changes/people-bulk-and-dashboard-depth/proposal.md) / [tasks](changes/people-bulk-and-dashboard-depth/tasks.md) |
| [Zitadel Actions v2 Deployment](changes/zitadel-actions-v2-deployment/) | 5 | Complete | [proposal](changes/zitadel-actions-v2-deployment/proposal.md) / [design](changes/zitadel-actions-v2-deployment/design.md) / [impl](changes/zitadel-actions-v2-deployment/IMPLEMENTATION.md) / [deploy](changes/zitadel-actions-v2-deployment/DEPLOY.md) |
| [Zitadel Event-Trigger Propagation](changes/zitadel-event-trigger-propagation/) | 5 | Complete | [proposal](changes/zitadel-event-trigger-propagation/proposal.md) / [design](changes/zitadel-event-trigger-propagation/design.md) / [tasks](changes/zitadel-event-trigger-propagation/tasks.md) / [impl](changes/zitadel-event-trigger-propagation/IMPLEMENTATION.md) |
| [Grant Expiration Scheduler](changes/grant-expiration-scheduler/) | 5 | Complete | [proposal](changes/grant-expiration-scheduler/proposal.md) / [design](changes/grant-expiration-scheduler/design.md) / [tasks](changes/grant-expiration-scheduler/tasks.md) |
| [Live Zitadel Data Source](changes/live-zitadel-data-source/) | 5 | Complete | [proposal](changes/live-zitadel-data-source/proposal.md) / [design](changes/live-zitadel-data-source/design.md) / [tasks](changes/live-zitadel-data-source/tasks.md) |
| [Live-Directory Identity Completeness](changes/live-directory-identity-completeness/) | 5 | Complete | [proposal](changes/live-directory-identity-completeness/proposal.md) / [design](changes/live-directory-identity-completeness/design.md) / [tasks](changes/live-directory-identity-completeness/tasks.md) |
| [Live-Only Production UI](changes/live-only-production-ui/) | 5 | Complete | [proposal](changes/live-only-production-ui/proposal.md) / [design](changes/live-only-production-ui/design.md) / [tasks](changes/live-only-production-ui/tasks.md) |
| [Dashboard UX Elevation](changes/dashboard-ux-elevation/) | 5 | Complete | [proposal](changes/dashboard-ux-elevation/proposal.md) / [design](changes/dashboard-ux-elevation/design.md) / [tasks](changes/dashboard-ux-elevation/tasks.md) |
| [Wave 1 — Production Trust Hardening](changes/wave-1-production-trust-hardening/) | 5.5 | In progress | [proposal](changes/wave-1-production-trust-hardening/proposal.md) / [design](changes/wave-1-production-trust-hardening/design.md) / [tasks](changes/wave-1-production-trust-hardening/tasks.md) |
| [Wave 2 · Part 1 — Frontend Palette Finalization](changes/wave-2-part-1-frontend-palette-finalization/) | 5.5 | In progress | [proposal](changes/wave-2-part-1-frontend-palette-finalization/proposal.md) / [design](changes/wave-2-part-1-frontend-palette-finalization/design.md) / [tasks](changes/wave-2-part-1-frontend-palette-finalization/tasks.md) |
| [Wave 2 · Part 2 — Backend Coherence](changes/wave-2-part-2-backend-coherence/) | 5.5 | In progress | [proposal](changes/wave-2-part-2-backend-coherence/proposal.md) / [design](changes/wave-2-part-2-backend-coherence/design.md) / [tasks](changes/wave-2-part-2-backend-coherence/tasks.md) |
| [Wave 2 · Part 3 — Operational Polish](changes/wave-2-part-3-operational-polish/) | 5.5 | In progress | [proposal](changes/wave-2-part-3-operational-polish/proposal.md) / [design](changes/wave-2-part-3-operational-polish/design.md) / [tasks](changes/wave-2-part-3-operational-polish/tasks.md) |
| [Wave 2 · Part 4 — Zitadel State Projection & Drift Control](changes/archive/2026-07-29-wave-2-part-4-zitadel-state-projection-and-drift-control/) | 5.5 | Archived | [proposal](changes/archive/2026-07-29-wave-2-part-4-zitadel-state-projection-and-drift-control/proposal.md) / [design](changes/archive/2026-07-29-wave-2-part-4-zitadel-state-projection-and-drift-control/design.md) / [tasks](changes/archive/2026-07-29-wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md) |
| [Wave 3 — Frontend Remainder & Consolidation](changes/archive/2026-07-29-wave-3-frontend-remainder-and-consolidation/) | 5.5 | Archived | [proposal](changes/archive/2026-07-29-wave-3-frontend-remainder-and-consolidation/proposal.md) / [design](changes/archive/2026-07-29-wave-3-frontend-remainder-and-consolidation/design.md) / [tasks](changes/archive/2026-07-29-wave-3-frontend-remainder-and-consolidation/tasks.md) |
| [July 2026 Audit Remediation](changes/july-2026-audit-remediation/) | 5.5 | Complete (sync items deferred) | [proposal](changes/july-2026-audit-remediation/proposal.md) / [tasks](changes/july-2026-audit-remediation/tasks.md) — backend authz at trust boundary, signed session cookie, stable webhook dedup, OE cuts; see AUDIT.md July addendum |
| [UI Capability Gap Closure](changes/ui-capability-gap-closure/) | 5.5 | In progress (A, B and C complete; C4/C5/C8 deferred with triggers recorded, C9b blocked on the hardware bridge) | [proposal](changes/ui-capability-gap-closure/proposal.md) / [design](changes/ui-capability-gap-closure/design.md) / [tasks](changes/ui-capability-gap-closure/tasks.md) — closes [docs/UI-CAPABILITY-GAPS.md](../docs/UI-CAPABILITY-GAPS.md) |
| [Bundle Versioning](changes/bundle-versioning/) | 5.5 | Implemented | [proposal](changes/bundle-versioning/proposal.md) / [design](changes/bundle-versioning/design.md) / [tasks](changes/bundle-versioning/tasks.md) — a bundle edit no longer cascades; publishing does, rehearsed |

## Roadmap Phase -> Change Mapping

| Phase | Status | Changes |
|-------|--------|---------|
| 1: UI Baseline | Complete | core-architecture, bun-migration |
| 2: Contract Hardening | Complete | contract-hardening |
| 3: Security Boundary | Complete | backend-onboarding, codebase-audit, zitadel-mgmt-client, webhook-listener, role-crud |
| 4: Infrastructure Bridge | In Progress | provisioning-intents, shadow-vault, sync-service |
| 5: Automation & Governance | In Progress | zitadel-actions-v2-deployment, grant-expiration-scheduler, live-zitadel-data-source, live-directory-identity-completeness, live-only-production-ui, dashboard-ux-elevation, zitadel-event-trigger-propagation; [remaining items](changes/mkauth-core-architecture/ROADMAP.md) |
| 5.5: Audit-Resolution Waves | In Progress | wave-1-production-trust-hardening, wave-2-part-1-frontend-palette-finalization, wave-2-part-2-backend-coherence, wave-2-part-3-operational-polish, wave-2-part-4-zitadel-state-projection-and-drift-control, wave-3-frontend-remainder-and-consolidation |
| 6: IdP Lifecycle | Not Started | [see Roadmap](changes/mkauth-core-architecture/ROADMAP.md) |
