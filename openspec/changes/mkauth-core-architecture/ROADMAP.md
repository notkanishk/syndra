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

## Phase 3: Orchestration Security Boundary (Immediate Production Gate)
*Objective: Close trust-boundary gaps before enabling broader live orchestration.*
- [x] **Container Split**: Docker Compose runs the frontend and backend as isolated services, with the UI proxying to the backend over the internal network.
- [/] **Frontend Session Auth**: Demo-backed cookie sessions now gate Admin/User view differentiation; live Zitadel OIDC remains the next step.
- [ ] **Per-Admin Backend Authorization**: Replace shared-API-key trust for privileged actions with backend-verified admin identity and authorization.
- [ ] **Production Data Plane Security**: Harden the Redis/action-injection perimeter, authentication, timeout behavior, and safe degraded responses.
- [ ] **Webhook Authenticity Validation**: Enforce production-grade verification for Zitadel-originated events before any downstream mutation or cache invalidation.
- [ ] **Zitadel Management Client**: Replace stubs with actual M2M Management API calls after the above controls are in place.
- [ ] **Live Webhook Listener**: Implement the real-time cache invalidator for validated Zitadel events.
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
