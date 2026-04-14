# MkAuth Development Roadmap

This document defines the high-level phases for the MkAuth implementation, transitioning from a conceptual design to a production-grade identity bridge.

## Phase 1: The UI Baseline (Current Status)
*Objective: Build the visual and conceptual foundation using a seeded catalog.*
- [x] **Demo Catalog**: Static users, projects, and apps for UI prototyping.
- [x] **Governance UI**: Mock request/approval flows and audit logging.
- [x] **Topology Visualizer**: SVG-based relationship graph.
- [x] **Claim Simulation**: Redis-backed cache for previewing JWT payloads.
- [x] **Logic Specifications**: Completion of the OpenSpec (this workspace).

## Phase 2: Contract Hardening and Test Foundation ✅ Complete
*Objective: Formalize schemas, validation, authorization boundaries, and backend-first regression coverage before expanding the live integration surface.*
- [x] **Contract Hardening**: `decodeJSONStrict` rejects unknown fields on all mutation endpoints; required-field, enum, duration, and idempotency guards implemented and tested. Injectable-dependency pattern established across all handlers and services.
- [x] **Backend-First Test Matrix**: 82 tests covering all critical mutation endpoints (bundles, rules, grants, access requests, governance, lineage, claim formatting, webhook, action injection, onboarding).
- [x] **Persistence Invariants**: Migrations 004 and 006 enforce status enums, positive durations, version bounds, resolution consistency, format_type enums, blank-name prevention, and expiry-after-create.
- [x] **Documentation Sync**: OpenSpec design, tasks, and roadmap updated to reflect implementation state and current risks.

## Phase 3: Orchestration Security Boundary (In Progress — next step: Zitadel Management Client)
*Objective: Close trust-boundary gaps before enabling broader live orchestration.*
- [x] **Container Split**: Docker Compose runs the frontend and backend as isolated services, with the UI proxying to the backend over the internal network.
- [x] **Frontend Session Auth**: PKCE authorization code flow implemented in Next.js without external libraries. `ui/src/lib/oidc.ts` handles PKCE crypto, token exchange, and Zitadel claim parsing. `ui/src/app/auth/zitadel/route.ts` initiates the flow; `ui/src/app/auth/callback/route.ts` exchanges the code and creates a session. The `mkauth_session` cookie uses a discriminated union (`demo | oidc`); OIDC sessions carry the raw access token and are forwarded as `Authorization: Bearer <token>` by both the proxy route and SSR server components. Demo users remain active when `ZITADEL_DOMAIN` is unset.
- [x] **Backend User-Token Authorization**: `withUserAuth` middleware validates Zitadel-issued RS256 JWTs (JWKS-backed, 1-hour cache) in production; falls back to API key in local-dev. Acting admin user ID stored in request context for audit attribution.
- [x] **Production Data Plane Security**: 50 ms Redis timeout; `claim_failure_mode` per project (`fail_closed` | `minimal_safe`); explicit `degradedResponse` for all failure paths; `pgx.ErrNoRows` vs real DB faults correctly distinguished; `[DATA PLANE]` structured logging.
- [x] **Webhook Authenticity Validation**: HMAC-SHA256 over `(X-Zitadel-Timestamp + "\n" + body)` — timestamp is part of the signed input preventing replay attacks; 5-minute freshness window enforced independently.
- [x] **Backend-Owned Onboarding Infrastructure**: `onboarding_triggers` table with idempotency key; `TriggerOnboarding` service records, assigns welcome bundle, and writes audit log; triggered by `role_key == "new_user"` on verified webhook; operator view at `GET /api/v1/onboarding/triggers`.
- [x] **HTTP Security Hardening** (Audit): Configurable CORS origin (replaces wildcard `*`), security response headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`), constant-time API key comparison, 1 MB request body limits on all endpoints.
- [x] **Local Dev Workflow** (Audit): `.env.example`, configurable `MIGRATION_PATH` for Docker-free operation, root `Makefile` with `dev`/`test`/`lint` targets.
- [x] **Backend Reliability** (Audit): `GET /healthz` health check endpoint, graceful shutdown with `signal.NotifyContext` and connection cleanup.
- [x] **Frontend Type Safety** (Audit): Generic typed API fetchers (`Promise<T>` replacing `Promise<any>`), shared `types.ts` mirroring Go models, Vitest test infrastructure with session module coverage.
- [x] **Cache Compiler Test Coverage** (Audit): Injectable deps pattern extended to `cache/` package; 5 tests covering empty grants, direct grants, mapping rule transitivity, bundle role inclusion, and fixed-point termination.
- [ ] **Zitadel Management Client**: Replace stubs with actual M2M Management API calls after frontend token forwarding is in place.
- [ ] **Live Webhook Listener**: Real-time cache invalidation from live Zitadel events (requires M2M client).
- [ ] **Advanced Role CRUD**: Implement "Snapshot & Fork" role cloning.

## Phase 4: The Infrastructure Bridge (Target: Hardware Sync)
*Objective: Enable legacy hardware support via LLDAP and Provisioning.*
- [ ] **Sync Service (Go)**: Build the dedicated concurrent provisioning worker.
- [ ] **LLDAP Integration**: Implement the `{project}_{role}` group mapping logic.
- [ ] **Shadow Password Vault**: Build the secure portal UI for setting Samba/LDAP secrets.
- [ ] **Provisioning Intents**: Implement the internal API contract between Backend and Sync Worker.

## Phase 5: Automation & Governance (Target: Operational Excellence)
*Objective: Eliminate manual overhead through policy-driven automation.*
- [ ] **Welcome Bundles**: Automatic role assignment for new Zitadel accounts.
- [ ] **Auto-Expiration**: Build the cleanup scheduler for temporary grants.
- [ ] **Advanced Filters**: Implement the multi-dimensional search engine for user management.
