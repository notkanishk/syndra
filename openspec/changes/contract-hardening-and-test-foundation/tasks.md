## 1. Define the hardening initiative

- [x] 1.1 Create a dedicated proposal for contract hardening and backend-first testing
- [x] 1.2 Document the target contract architecture and quality principles
- [x] 1.3 Record the current contract and spec drift that must be resolved

## 2. Define standards and coverage

- [x] 2.1 Add a capability spec for contract-quality requirements
- [x] 2.2 Add a capability spec for backend API validation and testing requirements
- [x] 2.3 Record the endpoint validation and test matrix for mission-critical backend flows

## 3. Align existing documents

- [x] 3.1 Update the roadmap so hardening and testing are explicitly the immediate next step
- [x] 3.2 Update the architecture design and coverage docs to reflect the hardening priority and current risks
- [x] 3.3 Update affected capability specs where the implementation and intended contract diverge

## 4. Implement Phase 2 hardening (complete)

- [x] 4.1 Extend injectable-dependency pattern to bundle and rules handlers (`handlers/deps.go`, `bundles.go`, `rules.go`)
- [x] 4.2 Extend injectable-dependency pattern to governance and lineage services (`services/deps.go`, `views.go`)
- [x] 4.3 Add idempotency guard to `handleResolveAccessRequest` — returns 409 `ALREADY_RESOLVED` when request status is already terminal
- [x] 4.4 Add nil-safety to `Governance` response so `PendingRequests` and `ExpiringGrants` serialize as `[]` not `null`
- [x] 4.5 Write handler tests for bundle CRUD — 13 tests covering empty name, whitespace name, unknown fields, idempotent assignment, audit attribution (`bundles_test.go`)
- [x] 4.6 Write handler tests for mapping rules — 8 tests covering missing fields, unknown fields, self-edge, cycle detection surfaced via handler, version increment, not-found (`rules_test.go`)
- [x] 4.7 Extend access flow tests — 8 new tests: expiry math for direct grants, zero-duration nil pointer, cache rebuild attribution, access request persistence and audit, idempotency 409 on already-resolved requests (`access_flow_test.go`)
- [x] 4.8 Write service-layer tests for governance and lineage — 9 new tests: nil-safe collections, pending count, unused bundle hints, source vs derived role labeling, bundle reason attribution, unknown user error, multi-hop derivation, unknown format type fallback (`views_test.go`)
- [x] 4.9 Add migration 006 with three new DB constraints: `ck_bundles_name_not_blank`, `ck_bundle_roles_not_blank`, `ck_direct_role_grants_expiry_after_create`
- [x] 4.10 Update spec docs to reflect implementation state

**Total test count after Phase 2: 82 tests (up from ~34)**
