# Syndra Development Roadmap

This document defines the high-level phases for the Syndra implementation, transitioning from a conceptual design to a production-grade identity bridge.

> **Navigation:** See [INDEX.md](../../INDEX.md) for the full spec graph and change-to-phase mapping.

## Phase 1: The UI Baseline (Current Status)
*Objective: Build the visual and conceptual foundation using a seeded catalog.*
*Implementing changes: [core-architecture](../syndra-core-architecture/proposal.md), [bun-native-migration](../bun-native-migration/proposal.md)*
- [x] **Demo Catalog**: Static users, projects, and apps for UI prototyping. → [demo-catalog spec](specs/demo-catalog/spec.md)
- [x] **Governance UI**: Mock request/approval flows and audit logging. → [access-governance spec](specs/access-governance/spec.md)
- [x] **Topology Visualizer**: SVG-based relationship graph. → [topology-graph spec](specs/topology-graph/spec.md)
- [x] **Claim Simulation**: Redis-backed cache for previewing JWT payloads. → [application-claims spec](specs/application-claims/spec.md)
- [x] **Logic Specifications**: Completion of the OpenSpec (this workspace).

## Phase 2: Contract Hardening and Test Foundation ✅ Complete
*Objective: Formalize schemas, validation, authorization boundaries, and backend-first regression coverage before expanding the live integration surface.*
*Implementing change: [contract-hardening-and-test-foundation](../contract-hardening-and-test-foundation/proposal.md)*
- [x] **Contract Hardening**: `decodeJSONStrict` rejects unknown fields on all mutation endpoints; required-field, enum, duration, and idempotency guards implemented and tested. Injectable-dependency pattern established across all handlers and services. → [contract-quality spec](../contract-hardening-and-test-foundation/specs/contract-quality/spec.md)
- [x] **Backend-First Test Matrix**: 82 tests covering all critical mutation endpoints (bundles, rules, grants, access requests, governance, lineage, claim formatting, webhook, action injection, onboarding). → [backend-api-testing spec](../contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md)
- [x] **Persistence Invariants**: Migrations 004 and 006 enforce status enums, positive durations, version bounds, resolution consistency, format_type enums, blank-name prevention, and expiry-after-create.
- [x] **Documentation Sync**: OpenSpec design, tasks, and roadmap updated to reflect implementation state and current risks.

## Phase 3: Orchestration Security Boundary ✅ Complete
*Objective: Close trust-boundary gaps before enabling broader live orchestration.*
*Implementing changes: [backend-onboarding](../backend-owned-onboarding-and-security-boundary/proposal.md), [codebase-audit](../codebase-audit-and-hardening/proposal.md), [zitadel-management-client](../zitadel-management-client/proposal.md), [live-webhook-listener](../live-webhook-listener/proposal.md), [advanced-role-crud](../advanced-role-crud/proposal.md)*
- [x] **Container Split**: Docker Compose runs the frontend and backend as isolated services, with the UI proxying to the backend over the internal network.
- [x] **Frontend Session Auth**: PKCE authorization code flow implemented in Next.js without external libraries. `ui/src/lib/oidc.ts` handles PKCE crypto, token exchange, and Zitadel claim parsing. `ui/src/app/auth/zitadel/route.ts` initiates the flow; `ui/src/app/auth/callback/route.ts` exchanges the code and creates a session. The `syndra_session` cookie uses a discriminated union (`demo | oidc`); OIDC sessions carry the raw access token and are forwarded as `Authorization: Bearer <token>` by both the proxy route and SSR server components. Demo users remain active when `ZITADEL_DOMAIN` is unset.
- [x] **Backend User-Token Authorization**: `withUserAuth` middleware validates Zitadel-issued RS256 JWTs (JWKS-backed, 1-hour cache) in production; falls back to API key in local-dev. Acting admin user ID stored in request context for audit attribution.
- [x] **Production Data Plane Security**: 50 ms Redis timeout; `claim_failure_mode` per project (`fail_closed` | `minimal_safe`); explicit `degradedResponse` for all failure paths; `pgx.ErrNoRows` vs real DB faults correctly distinguished; `[DATA PLANE]` structured logging.
- [x] **Webhook Authenticity Validation**: HMAC-SHA256 over `(X-Zitadel-Timestamp + "\n" + body)` — timestamp is part of the signed input preventing replay attacks; 5-minute freshness window enforced independently.
- [x] **Backend-Owned Onboarding Infrastructure**: `onboarding_triggers` table with idempotency key; `TriggerOnboarding` service records, assigns welcome bundle, and writes audit log; triggered by `role_key == "new_user"` on verified webhook; operator view at `GET /api/v1/onboarding/triggers`.
- [x] **HTTP Security Hardening** (Audit): Configurable CORS origin (replaces wildcard `*`), security response headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`), constant-time API key comparison, 1 MB request body limits on all endpoints.
- [x] **Local Dev Workflow** (Audit): `.env.example`, configurable `MIGRATION_PATH` for Docker-free operation, root `Makefile` with `dev`/`test`/`lint` targets.
- [x] **Backend Reliability** (Audit): `GET /healthz` health check endpoint, graceful shutdown with `signal.NotifyContext` and connection cleanup.
- [x] **Frontend Type Safety** (Audit): Generic typed API fetchers (`Promise<T>` replacing `Promise<any>`), shared `types.ts` mirroring Go models, Vitest test infrastructure with session module coverage.
- [x] **Cache Compiler Test Coverage** (Audit): Injectable deps pattern extended to `cache/` package; 5 tests covering empty grants, direct grants, mapping rule transitivity, bundle role inclusion, and fixed-point termination.
- [x] **Zitadel Management Client**: Direct HTTP client for Zitadel Management API v1 using JWT profile M2M auth (RS256 assertion, token caching, retry with backoff). Implements `AddUserGrant`, `RemoveUserGrant`, `ListUserGrants`, `GetUser`. Zero new dependencies — reuses `golang-jwt/jwt/v5` and stdlib `net/http`. Graceful degradation to local-policy-only mode when credentials absent. 22 tests covering key loading, token lifecycle, all API methods, retry logic, and orchestrator integration.
- [x] **Live Webhook Listener**: Event-type-aware webhook dispatch (6 event types), event persistence with idempotency deduplication, reverse orchestration (`RevokeMappingRules`) for grant revocations, user deactivation handling, operator endpoint (`GET /api/v1/webhook/events`). 12 new tests. Backward compatible — `event_type` defaults to `grant_added` when absent.
- [x] **Advanced Role CRUD**: Role creation with Zitadel propagation, Snapshot & Fork cloning, global role catalog with usage metrics and unused detection. 18 new tests. Migration 000008.

## Phase 4: The Target Plane (was: The Infrastructure Bridge)
*Objective: reach the systems that do not speak OIDC.*

**The LLDAP path is abandoned.** Change [`addon-platform`](../addon-platform/proposal.md)
(2026-08-10) replaced it: Syndra reaches TrueNAS SCALE — and whatever comes
next — through each target's own management API, from an add-on container per
target, instead of reflecting identity into an intermediate directory nobody
else was going to read. `sync/`, the provisioning-intent queue and the
Argon2id password vault are deleted.

The items below are kept rather than removed. A track that vanishes reads as
forgotten; this one was decided.
*Implementing change: [addon-platform](../addon-platform/proposal.md). Superseded: [provisioning-intents](../provisioning-intents/proposal.md), [shadow-password-vault](../shadow-password-vault/proposal.md), [sync-service](../sync-service/proposal.md)*
- [x] **Provisioning Intents**: Backend-side intent emission on grant changes, LLDAP group flattening, sync service polling API. Migration 000009. 22 new tests. → [provisioning spec](specs/provisioning/spec.md)
- [x] **Shadow Password Vault**: Argon2id-hashed infrastructure-only credentials for Samba/LLDAP, self-service set/clear/status API, dedicated audit trail, sync service hash retrieval. Migration 000010. 23 new tests. → [ldap-sync spec](specs/ldap-sync/spec.md)
- [x] **Sync Service (Go)**: Independent Docker container polling backend intents, executing LLDAP group mutations via `go-ldap/v3`, syncing shadow passwords, per-UID ordering, auto-reconnect. User profile endpoint for displayName/mail provisioning. 32 sync tests + backend profile endpoint. → [sync-service design](../sync-service/design.md)
- [~] **LLDAP Integration** — *abandoned with the bridge.* Originally: End-to-end wiring against the real external LLDAP deployment, reconciliation loop, and production connectivity validation for an LLDAP server hosted outside the Syndra Compose stack (for example, a separate Proxmox LXC). This item is currently paused pending research on real LLDAP compatibility for password propagation and credential handling semantics. → [sync-service open questions](../sync-service/design.md)

## Phase 5: Automation & Governance (Target: Operational Excellence)
*Objective: Eliminate manual overhead, close safety gaps, and harden operational posture.*

### Automation
- [x] **Welcome Bundle Configuration**: shipped — migration `000012_welcome_bundle_flag`, the `is_welcome` column, operator-gated `PUT /api/v1/bundles/{id}/welcome`, and the toggle on the bundles page. The convention-based name matching this described replacing is gone. → [automation-policies spec](specs/automation-policies/spec.md)
- [x] **Grant Expiration Scheduler**: Background worker (`backend/internal/services/expiry`) sweeps expired direct grants every `EXPIRY_SCHEDULER_INTERVAL` (default 5m), computes the closure delta and queues whatever it revokes — upstream and on every mapped target — hard-deletes rows, invalidates the user cache, writes `direct_grant.revoked_by_expiry` audit entries, and best-effort cascades derived Zitadel grants. 14 new tests. → [grant-expiration-scheduler](../grant-expiration-scheduler/) / [access-governance spec](specs/access-governance/spec.md)
- [x] **Live Zitadel Data Source**: `backend/internal/directory/` provides a `Source` seam that swaps the admin UI's users/projects/roles backing from the hardcoded demo catalog to the live Zitadel Management API when `ZITADEL_DOMAIN` + `ZITADEL_MACHINE_KEY_PATH` are configured. Includes a 30s TTL cache, targeted invalidation on mutation, an overlay over `claim_profiles` for application metadata, and seed-skip-in-live-mode behavior. Frontend contracts unchanged; 10 new tests. → [live-zitadel-data-source](../live-zitadel-data-source/) / [user-management spec](specs/user-management/spec.md)
- [x] **Live-Directory Identity Completeness**: `/applications` renders real Zitadel applications (OIDC / API / SAML) via the new `ListApplications` client method — previously fabricated one-per-project. `/users` populates `Title` / `Team` from Zitadel user metadata (`ListUserMetadata`, well-known keys `title`/`team`). Bounded-parallel fan-out (errgroup limit 4 for apps, 8 for metadata), per-entity failure tolerance so one bad project/user doesn't blank the whole page. Frontend contracts unchanged; 10 new tests. → [live-directory-identity-completeness](../live-directory-identity-completeness/) / [application-claims spec](specs/application-claims/spec.md) / [user-management spec](specs/user-management/spec.md)
- [ ] **Service Catalog Abstraction**: Close the gap between the spec'd service-to-bundle request mapping and the current project/role fallback. → [service-catalog spec](specs/service-catalog/spec.md)

### Consistency & Safety
- [x] **Zitadel Reconciliation**: Periodic drift detection between Syndra local grant/role state and Zitadel's actual state, with operator-visible reports and optional auto-correction. Closes out Phase 5.5's Wave 2 · Part 4 (all 3 sub-phases): scheduled (`DRIFT_RECONCILIATION_INTERVAL_HOURS`) + webhook drift detection, operator triage UI (`/governance/drift`), and bundle/rule cascade projection through a confirmable outbox (`auto`/`manual` `confirmation_mode`, `/operations/cascades`). → [wave-2-part-4](../wave-2-part-4-zitadel-state-projection-and-drift-control/) / [drift-control row](specs/feature-coverage.md)
- [~] **LLDAP Reconciliation** — *replaced by the add-on reconcile*, which resolves current state and queues a convergence rather than overwriting drift from a mirror. Originally: Periodic full-sync comparing Syndra provisioning state against LLDAP group memberships, overwriting LLDAP drift per one-way authority rule. → [ldap-sync spec](specs/ldap-sync/spec.md)
- [ ] **Partial Failure Rollback**: Compensating revocations in `EnforceMappingRules` and `RevokeMappingRules` when Zitadel API calls partially fail. Currently best-effort log-and-continue. → [provisioning spec](specs/provisioning/spec.md)

### Admin UX
- [ ] **Advanced Filters**: Multi-dimensional user search (by project, role, account age, grant staleness). → [user-management spec](specs/user-management/spec.md)
- [ ] **Bulk Operations**: Mass grant/revoke with preview, per-user outcomes, and idempotent retry. → [access-governance spec](specs/access-governance/spec.md)

### Operations
- [ ] **Rate Limiting**: Request throttling for webhook, action-injection, and shadow-password endpoints. Requires design decision (in-process vs Redis-backed).
- [ ] **Observability**: Metrics, alerting thresholds, and operational dashboard integration beyond current structured logs.
- [x] **Actions v2 Deployment**: Zitadel Actions v2 target configuration, HMAC-verified `/api/action/inject` endpoint, and failure-mode smoke test in repo. Handler reshaped to the real v2 envelope (`append_claims[{key,value}]`), correcting prior v1 wording (`SetCustomClaims`). Target manifest, `register.sh`, signing-key capture, and `DEPLOY.md` shipped under `zitadel/actions/`. → [actions-v2-deployment](../zitadel-actions-v2-deployment/) / [application-claims spec](specs/application-claims/spec.md)
- [x] **Event-Trigger Propagation**: Lifecycle event delivery via Actions v2 `condition.event` triggers — second target (`syndra-event-listener`, `restAsync`) registered alongside the claim injector by a single `make zitadel-actions-register`. `/api/webhooks/zitadel` migrated to canonical `withZitadelActionSignature`/`ZITADEL_EVENT_SIGNING_KEY` (legacy `ZITADEL_WEBHOOK_SECRET` retired). New translator maps `user.human.{added,deactivated,locked}` and `user.grant.{added,changed,removed}` to internal `WebhookPayload` (with multi-role `role_keys[]`); self-mutation echoes suppressed via `ZITADEL_M2M_USER_ID`. Closes the producer-side gap on welcome-bundle assignment, mapping-rule cascade, cache invalidation, and the convergences a role change queues for its mapped targets. → [zitadel-event-trigger-propagation](../zitadel-event-trigger-propagation/) / [lifecycle-event-propagation spec](../zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md)
- [ ] **CI/CD Pipeline**: Automated test runs, migration validation, container build verification.

## Phase 6: IdP Lifecycle (Target: Account Lifecycle Integrity)
*Objective: Ensure upstream Google Workspace account state propagates through the full identity chain.*

Google Workspace is the sole IdP. Zitadel does not currently auto-detect when a Google Workspace account is suspended or deleted. A dedicated service is needed to close this gap.

- [ ] **Google Workspace Account Poller**: A separate service (dedicated Docker container) that monthly polls Google Workspace via the Admin SDK Directory API to verify all Zitadel users still have active Google accounts. Suspended or deleted accounts trigger user deactivation in Zitadel via the Management API, which then cascades through Syndra's existing webhook pipeline (`user_deactivated` -> cache invalidation -> the closure diff -> a convergence queued for every mapped target). → [design.md section 10](design.md)
- [ ] **Optional: ⌘K command palette**: Struck from the current spec during the May 2026 audit resolution (design.md §5 no longer lists it). Reserved here as an optional Phase 6 nicety if operator navigation demand materializes.
