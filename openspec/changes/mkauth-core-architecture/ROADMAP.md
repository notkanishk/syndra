# MkAuth Development Roadmap

This document defines the high-level phases for the MkAuth implementation, transitioning from a conceptual design to a production-grade identity bridge.

## Phase 1: The UI Baseline (Current Status)
*Objective: Build the visual and conceptual foundation using a seeded catalog.*
- [x] **Demo Catalog**: Static users, projects, and apps for UI prototyping.
- [x] **Governance UI**: Mock request/approval flows and audit logging.
- [x] **Topology Visualizer**: SVG-based relationship graph.
- [x] **Claim Simulation**: Redis-backed cache for previewing JWT payloads.
- [x] **Logic Specifications**: Completion of the OpenSpec (this workspace).

## Phase 2: The Orchestration Core (Target: Digital Parity)
*Objective: Transition from mock data to real Zitadel integration and container isolation.*
- [x] **Container Split**: Docker Compose runs the frontend and backend as isolated services, with the UI proxying to the backend over the internal network.
- [/] **Frontend Session Auth**: Demo-backed cookie sessions now gate Admin/User view differentiation; live Zitadel OIDC remains the next step.
- [ ] **Zitadel Management Client**: Replace stubs with actual M2M Management API calls.
- [ ] **Live Webhook Listener**: Implement the real-time cache invalidator for Zitadel events.
- [ ] **Advanced Role CRUD**: Implement "Snapshot & Fork" role cloning.

## Phase 3: The Infrastructure Bridge (Target: Hardware Sync)
*Objective: Enable legacy hardware support via LLDAP and Provisioning.*
- [ ] **Sync Service (Go)**: Build the dedicated concurrent provisioning worker.
- [ ] **LLDAP Integration**: Implement the `{project}_{role}` group mapping logic.
- [ ] **Shadow Password Vault**: Build the secure portal UI for setting Samba/LDAP secrets.
- [ ] **Provisioning Intents**: Implement the internal API contract between Backend and Sync Worker.

## Phase 4: Automation & Governance (Target: Operational Excellence)
*Objective: Eliminate manual overhead through policy-driven automation.*
- [ ] **Welcome Bundles**: Automatic role assignment for new Zitadel accounts.
- [ ] **Auto-Expiration**: Build the cleanup scheduler for temporary grants.
- [ ] **Advanced Filters**: Implement the multi-dimensional search engine for user management.
- [ ] **Security Hardening**: Perform a full audit of the internal M2M auth and Redis perimeter.
