# MkAuth Development Roadmap

This document defines the high-level phases for the MkAuth implementation, transitioning from a conceptual design to a production-grade identity bridge.

## Phase 1: The UI Baseline (Current Status)
*Objective: Build the visual and conceptual foundation using a seeded catalog.*
- [x] **Demo Catalog**: Static users, projects, and apps for UI prototyping.
- [x] **Governance UI**: Mock request/approval flows and audit logging.
- [x] **Topology Visualizer**: SVG-based relationship graph.
- [x] **Claim Simulation**: Redis-backed cache for previewing JWT payloads.
- [x] **Logic Specifications**: Completion of the OpenSpec (this workspace).

## Phase 2: Contract Hardening and Test Foundation (Immediate Next Step)
*Objective: Formalize schemas, validation, authorization boundaries, and backend-first regression coverage before expanding the live integration surface.*
- [x] **Contract Hardening**: Introduce purpose-built request/response/domain types, strict validation, and durable API error contracts.
- [x] **Backend-First Test Matrix**: Add full unit and handler coverage for mission-critical backend flows, especially mapping rules, grants, access requests, claims, and webhooks.
- [x] **Persistence Invariants**: Strengthen database constraints so critical domain rules are enforced below the application layer as well.
- [x] **Documentation Sync**: Keep OpenSpec, roadmap, and coverage docs aligned with the hardened contracts and any drift corrections.

## Phase 3: Orchestration Security Boundary (In Progress — backend controls complete, UI token forwarding pending)
*Objective: Close trust-boundary gaps before enabling broader live orchestration.*
- [x] **Container Split**: Docker Compose runs the frontend and backend as isolated services, with the UI proxying to the backend over the internal network.
- [/] **Frontend Session Auth**: Demo-backed cookie sessions gate Admin/User view differentiation; live Zitadel OIDC and token forwarding from UI to backend remain the next step.
- [x] **Backend User-Token Authorization**: `withUserAuth` middleware validates Zitadel-issued RS256 JWTs (JWKS-backed, 1-hour cache) in production; falls back to API key in local-dev. Acting admin user ID stored in request context for audit attribution.
- [x] **Production Data Plane Security**: 50 ms Redis timeout; `claim_failure_mode` per project (`fail_closed` | `minimal_safe`); explicit `degradedResponse` for all failure paths; `pgx.ErrNoRows` vs real DB faults correctly distinguished; `[DATA PLANE]` structured logging.
- [x] **Webhook Authenticity Validation**: HMAC-SHA256 over `(X-Zitadel-Timestamp + "\n" + body)` — timestamp is part of the signed input preventing replay attacks; 5-minute freshness window enforced independently.
- [x] **Backend-Owned Onboarding Infrastructure**: `onboarding_triggers` table with idempotency key; `TriggerOnboarding` service records, assigns welcome bundle, and writes audit log; triggered by `role_key == "new_user"` on verified webhook; operator view at `GET /api/v1/onboarding/triggers`.
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
