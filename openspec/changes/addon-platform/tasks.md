## 1. Target dimension in the data layer

- [ ] 1.1 Migration: create the `targets` registry table seeded with `zitadel`; add `target TEXT NOT NULL DEFAULT 'zitadel'` to `pending_zitadel_propagations`, `direct_role_grants`, and the drift tables with a foreign key to it and an index on `(target, status)`
- [ ] 1.2 Migration-coherence guard test asserting the registry table, the new columns, defaults, foreign keys, and index exist, that pre-existing rows read back as `zitadel`, and that a row naming an unregistered target is refused
- [ ] 1.3 Thread `target` through the propagation claim, drain, and terminal-state writes in `services/propagation`; the drain claims one target per pass
- [ ] 1.4 Tests: a drain for one target leaves other targets' rows untouched; an unreachable target halts only its own pass
- [ ] 1.5 Thread `target` through `services/drift` sweep and `drift_triage.go` so drift is classified per target
- [ ] 1.6 Tests: drift on one target does not appear under another; existing Zitadel drift behaviour is unchanged
- [ ] 1.7 Thread `target` through `services/expiry` drain so allowance and grant expiry re-converge the right target
- [ ] 1.8 Tests: expiry sweep emits per-target propagations and leaves unrelated targets alone
- [ ] 1.9 Migration: `desired_state_snapshots` — immutable rows per `(subject, target)` with a monotonic version; outbox rows reference a snapshot instead of instructing a re-resolve
- [ ] 1.10 Migration-coherence guard test: snapshot rows are immutable, versions are monotonic per `(subject, target)`, and the outbox foreign key holds
- [ ] 1.11 Per-`(subject, target)` serialization and stale-version rejection in the drain
- [ ] 1.12 Tests: an apply carrying an older version is rejected without dispatch; concurrent applies for one subject serialize; the settled state equals the higher version; a queued grant cannot land after a later revoke
- [ ] 1.13 Two snapshot read paths: operator-initiated applies dispatch the recorded snapshot, periodic reconcile resolves current state
- [ ] 1.14 Tests: a policy change between approval and drain still applies the approved snapshot; reconcile resolves fresh and never replays a superseded snapshot
- [ ] 1.15 Revocation priority: the drain dispatches revocations before grants for the same target
- [ ] 1.16 Tests: a mixed queue dispatches revocations first; retry budget applies per row without starving revocations

## 2. Add-on registry and wire contract

- [ ] 2.1 Define the manifest types: entitlement schema, operation descriptors (`scope`, `confirm`, `secret_params`), target product and version
- [ ] 2.2 Add-on registry: config-driven registration (base URL, shared secret ref), manifest fetch and cache, `deps.go` seam for tests
- [ ] 2.3 Tests: unregistered add-on is not callable; an operation absent from the manifest is rejected even if requested
- [ ] 2.4 Add-on HTTP client with per-add-on auth, timeouts, and a circuit breaker; carries plan id, fingerprints, and operation id on every mutating call
- [ ] 2.5 Tests: client failure modes — unreachable, timeout, 5xx, open circuit — each map to the intended outcome and never to silent success
- [ ] 2.6 Redaction layer keyed off `secret_params`: values stripped before any audit write, log line, or outbox payload
- [ ] 2.7 Tests: a secret parameter value never appears in audit rows, outbox rows, or emitted logs, asserted by scanning the written records
- [ ] 2.8 Enqueue path for entitlement changes: outbox row plus audit row in one transaction with `target` set, before any add-on call
- [ ] 2.9 Tests: transactional rollback leaves no partial state; no add-on call is issued before commit
- [ ] 2.10 Migration: `addon_operations` record table — operation id, target FK, actor, subject, operation name, status, timestamps; no parameter values column
- [ ] 2.11 Migration-coherence guard test for `addon_operations`, including that there is no column able to hold a secret parameter value
- [ ] 2.12 Secret-bearing dispatch protocol: commit the operation record with a non-terminal status before the call, send the operation id, write the terminal status after the response; never enqueue in the outbox and never auto-retry
- [ ] 2.13 Tests: the record commits before the call; a simulated crash between dispatch and terminal write leaves the row non-terminal and no retry is attempted; no secret value reaches any table or log
- [ ] 2.14 Unresolved-operation surface: non-terminal `addon_operations` rows presented as unresolved, distinct from succeeded and failed
- [ ] 2.15 Tests: an unresolved row renders as unresolved and is excluded from both success and failure counts
- [ ] 2.16 Plan store: `POST /plan` handler persisting a plan id with bounded TTL, per-subject desired state and target-state fingerprint
- [ ] 2.17 Tests: a plan persists and expires; an apply citing an unknown or expired plan id is rejected
- [ ] 2.18 Apply gate: reject any apply not citing a backend-issued plan id; re-verify every fingerprint against live target state before dispatch
- [ ] 2.19 Tests: a client-supplied plan is refused; a fingerprint mismatch fails the apply, mutates nothing, and names the subjects that moved
- [ ] 2.20 Queued accounting: extend the existing `BulkSummary.Queued` semantics to add-on targets so unconfirmed rows never count as succeeded
- [ ] 2.21 Tests: an unreachable add-on yields queued rows, a reachable one yields succeeded rows, and the summary distinguishes them
- [ ] 2.22 Backend operation policy: per-operation-id scope, confirmation requirement, and parameter schema, owned by the backend and independent of any manifest
- [ ] 2.23 Manifest intersection: effective operation set is manifest ∩ policy with policy prevailing; unknown operation ids fail closed
- [ ] 2.24 Tests: a manifest cannot widen scope, cannot drop a confirmation requirement, cannot introduce an unknown operation, and can narrow
- [ ] 2.25 mTLS between backend and add-ons with a private CA, or signed requests carrying timestamp and body hash where mTLS is impractical
- [ ] 2.26 Tests: a call without a valid client certificate is refused; a signature not matching body or timestamp is refused; a bare shared secret is insufficient
- [ ] 2.27 Certificate and key material provisioning, rotation, and expiry surfacing for the add-on transport
- [ ] 2.28 Tests: an expiring transport credential is surfaced before it fails; rotation does not drop in-flight operations

## 3. Zitadel plan retrofit

- [ ] 3.1 Persist plans on the Zitadel rehearse paths: `rehearseDriftBatch`, bulk, and reconciliation return a plan id alongside the outcomes
- [ ] 3.2 Fingerprint a Zitadel subject as a hash of their current grant set, recorded per subject in the plan
- [ ] 3.3 Tests: a plan persists with per-subject fingerprints and expires on its TTL
- [ ] 3.4 **BREAKING** Change `applyDriftPlan` and the bulk and reconciliation apply endpoints to take a plan id instead of a plan body; re-verify fingerprints before dispatch
- [ ] 3.5 Tests: an apply carrying a plan body is refused; an unknown or expired plan id is refused; a subject whose grants moved between plan and apply fails the apply, mutates nothing, and is named in the error
- [ ] 3.6 Update the frontend bulk, drift-triage, and reconciliation flows to hold and send the plan id, with the stale-plan re-plan path
- [ ] 3.7 Tests: the UI sends only the plan id; a stale plan re-plans and shows which subjects moved rather than reporting a generic failure

## 4. TrueNAS add-on: skeleton and read path

- [ ] 4.1 New Go module `addons/truenas` with Dockerfile and Compose service, internal network only, no published port
- [ ] 4.2 TrueNAS client wrapper over `github.com/truenas/api_client_golang`: single persistent session, `auth.login_with_api_key`, reconnect
- [ ] 4.3 Tests: session is reused across calls; a rate-limit response opens the circuit instead of retrying
- [ ] 4.4 `GET /health`: reachability, `system.version` probe, key expiry, last successful read, read-only flag, circuit state
- [ ] 4.5 Tests: unsupported target major version refuses mutations and reports the supported range through health
- [ ] 4.6 `GET /capabilities`: manifest with entitlement schema and the phase-1 operation set, each operation carrying an availability flag and a reason when unavailable
- [ ] 4.7 Per-operation compatibility probe: mark operations unavailable when the target lacks the method or capability they depend on
- [ ] 4.8 `GET /subjects`: state read via `user.query` and `group.query` with an explicit `select`, feeding the drift sweep
- [ ] 4.9 Tests: hash fields are absent from every response, snapshot, and log entry — asserted by scanning serialized output
- [ ] 4.10 Tests: an operation whose target method is absent is reported unavailable with a reason and is refused rather than attempted
- [ ] 4.11 Local stores: bbolt idempotency bucket with TTL, snapshot bucket; no command queue
- [ ] 4.12 Tests: the snapshot serves a stale read with its age when the target is unreachable
- [ ] 4.13 Mutation log: append-only JSONL, `0600`, fsync before the operation is reported complete, size-based rotation with bounded retention, each record carrying the digest of the previous
- [ ] 4.14 Tests: a record is durable before completion is reported; altering or removing a record breaks chain verification; a secret-bearing mutation logs the event without the value
- [ ] 4.15 Lifecycle state flag `active | draining | read_only`, settable without redeploy and reported through health
- [ ] 4.16 Tests: `read_only` refuses all mutations while serving reads; `draining` refuses new mutations while in-flight ones settle and reports when drained; neither is reported as unhealthy

## 5. TrueNAS add-on: entitlement plane

- [ ] 5.1 `POST /apply`: accept a resolved entitlement set for one subject, converge via `user.update({groups})`, level-triggered
- [ ] 5.2 Tests: re-applying an unchanged set is a no-op with no mutating call; a reduced set converges to exactly the remaining groups
- [ ] 5.3 Blast-radius limiter: compute affected subject count, refuse beyond the configured limit without an explicit scope acknowledgement
- [ ] 5.4 Tests: an oversized effect is refused, returns the computed count, and mutates nothing
- [ ] 5.5 `POST /plan`: returns `BulkPlan`/`BulkOutcome`-shaped outcomes with `Detail` and `Consequence`, plus a per-subject target-state fingerprint, mutating nothing
- [ ] 5.6 Tests: planning issues no mutating call, returns the apply path's shape, and produces a fingerprint that changes when the subject's target state changes
- [ ] 5.7 Fingerprint re-verification on apply: refuse the call if any supplied fingerprint no longer matches live target state
- [ ] 5.8 Tests: a subject mutated out of band between plan and apply causes refusal with that subject named, and nothing is applied
- [ ] 5.9 Operation-id deduplication for `/apply` and `/op/{id}`, backed by the idempotency store
- [ ] 5.10 Tests: replaying an operation id returns the original outcome without a second mutating call
- [ ] 5.11 Absence handling: a subject missing from the expected set is reported as drift and never deleted or locked
- [ ] 5.12 Tests: a missing subject produces a drift report and no mutation

## 6. TrueNAS add-on: operations

- [ ] 6.1 `account.ensure`: query-then-create with a deterministic username derivation, report the created account name back, idempotency-keyed
- [ ] 6.2 Tests: repeat invocation creates no duplicate account and returns the same account name
- [ ] 6.3 Username derivation from the email localpart: lowercase, strip sub-addressing, replace characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/`, enforce a valid leading character, truncate to 32; collision suffix from a stable hash of the Zitadel user ID
- [ ] 6.4 Tests: derivation is deterministic and always pattern-valid; non-ASCII, sub-addressed, leading-dot, and over-length localparts normalize; a forced collision resolves reproducibly and never reuses another subject's name
- [ ] 6.5 Normalization fallbacks: a localpart that normalizes to nothing usable falls back to a name derived from the Zitadel user id; the collision suffix is reserved before truncation, never appended after
- [ ] 6.6 Tests: an all-invalid and an empty-after-normalization localpart both yield deterministic valid names; a name needing both truncation and a suffix stays within the limit and still disambiguates
- [ ] 6.7 Binding conflict handling: an unbound account already holding the derived name halts the operation and is reported for an operator decision — adopt or disambiguate — never silently adopted
- [ ] 6.8 Tests: a colliding unbound account causes no create, adopt, or modify, and surfaces the existing account for decision
- [ ] 6.9 Binding recovery in reconcile: an account whose name changed out of band beneath a recorded binding is reported as a rename, not as a missing account
- [ ] 6.10 Tests: an out-of-band rename reports against the existing binding and creates no replacement account
- [ ] 6.11 Record the derived name against the subject as the authoritative binding; a later email change MUST NOT rename an existing account
- [ ] 6.12 Tests: an email change after creation leaves the account name unchanged and the binding still resolves the subject
- [ ] 6.13 `password.set`: `user.update({password})`, forwarded and never persisted, declared with `secret_params`
- [ ] 6.14 Tests: the plaintext appears in no store, snapshot, or log; the mutation log records that a password was set
- [ ] 6.15 `account.lock`: set `locked`, clear `smb`, rotate password; response states that established sessions end on reconnect
- [ ] 6.16 Tests: lock applies all three effects and the response carries the reconnect caveat
- [ ] 6.17 `account.smb.set`: reversible SMB suspend and resume
- [ ] 6.18 Tests: suspend then resume restores the prior state
- [ ] 6.19 `account.purge`: plan discloses retained home data before apply; `user.delete` only on explicit confirmation
- [ ] 6.20 Tests: purge without confirmation refuses; the plan reports retained data size
- [ ] 6.21 `activity.get`: `audit.query` with `service: "SMB"`, reporting shares with auditing disabled
- [ ] 6.22 Tests: an empty result names the unaudited shares rather than implying no activity
- [ ] 6.23 `health.get`: `system.info`, `alert.list`, `pool.query`, `service.query` composed into the operator health shape
- [ ] 6.24 Tests: health composes all four sources and degrades per-source rather than failing whole

## 7. Role-to-target mapping and lifecycle

- [ ] 7.1 Migration: `target_role_mappings` binding `(target, project_id, role_key)` to an entitlement field and value, versioned with actor and reason like bundle definitions
- [ ] 7.2 Migration-coherence guard test: uniqueness on `(target, project_id, role_key, field)`, version history retained, rollback target reachable
- [ ] 7.3 Mapping CRUD with split validation — backend checks the field is in the add-on's declared schema and the role exists; the add-on confirms the value resolves on its target
- [ ] 7.4 Tests: an undeclared field is rejected without calling the add-on; an unresolvable value is rejected after the add-on reports it; a duplicate binding is rejected
- [ ] 7.5 Mapping versioning and rollback reusing the bundle version machinery
- [ ] 7.6 Tests: an edit creates a new version with actor and time; rollback restores the prior mapping and re-resolves affected subjects
- [ ] 7.7 Role-derived resolution: derive the role half of a subject's entitlement set from the mappings and from nothing else
- [ ] 7.8 Tests: a role with no mapping contributes nothing; two mappings on one role contribute both fields; resolution is stable under repetition
- [ ] 7.9 Lifecycle trigger on the existing grant path: look up targets mapped to the changed role, enqueue `account.ensure` on gaining a first mapped role, enqueue `account.lock` on losing the last
- [ ] 7.10 Tests: first mapped role creates the account before the entitlement apply; last mapped role removal locks and never deletes; regaining a mapped role restores without operator action; an unmapped role triggers nothing
- [ ] 7.11 Mapping edit and delete plan through the standard plan path, subject to the blast-radius guard
- [ ] 7.12 Tests: deleting a mapping held by many subjects plans across all of them and trips the blast-radius guard without an acknowledgement

## 8. Allowances

- [ ] 8.1 Migration: allowance table with subject, target, entitlement field, value, direction, actor, reason, `expires_at`
- [ ] 8.2 Migration-coherence guard test, including the CHECK that a subtractive allowance requires a non-null `expires_at`
- [ ] 8.3 Resolver: combine role-derived entitlements with additive and subtractive allowances into the resolved set handed to the add-on
- [ ] 8.4 Tests: additive extends, subtractive removes, deny beats allow, and the resolved set is stable under re-resolution
- [ ] 8.5 Reject subtractive allowances submitted without an expiry, with an error directing the operator to the role mapping
- [ ] 8.6 Tests: submission without expiry is rejected; with expiry is accepted
- [ ] 8.7 Expiry sweep removes lapsed allowances and re-converges the subject, writing an audit entry
- [ ] 8.8 Tests: a lapsed subtractive allowance restores role-derived access and records the restoration
- [ ] 8.9 Extend the lineage builder in `services/views.go` with the allowance band, carrying actor and grant time
- [ ] 8.10 Tests: every entitlement attributes to exactly one of source, derived, or allowance

## 9. Operator surfaces

- [ ] 9.1 Manifest-driven operation rendering: member and admin panels built from `scope`, with no add-on-specific frontend code
- [ ] 9.2 Tests: an operation removed from a manifest disappears from the UI without a frontend change
- [ ] 9.3 Plan-then-apply flow reusing the `rehearse* → apply*` pattern, carrying the backend-issued plan id rather than round-tripping the plan body
- [ ] 9.4 Tests: apply is unreachable until a plan has been issued; the request carries only the plan id; a tampered or client-fabricated plan body is refused
- [ ] 9.5 Stale-plan recovery UX: a rejected apply re-plans and shows which subjects moved since the operator reviewed it
- [ ] 9.6 Tests: a subject changed between plan and apply produces the stale-plan path with that subject named, not a generic failure
- [ ] 9.7 Mapping management UI: role-to-target bindings with version history, rollback, and the plan shown before any edit or delete lands
- [ ] 9.8 Tests: an edit affecting many subjects shows the full plan and refuses without the blast-radius acknowledgement; rollback restores the prior version
- [ ] 9.9 Unconfirmed-revocation surface beside drift triage, with age-threshold escalation to a security-finding presentation
- [ ] 9.10 Tests: queued revokes are counted and presented apart from queued grants; crossing the threshold changes the presentation
- [ ] 9.11 Dormant-account housekeeping view: reason, age, individual and bulk action, plan before apply
- [ ] 9.12 Tests: accounts held by an active role are excluded; bulk action plans before applying
- [ ] 9.13 Add-on health surface distinguishing unreachable, read-only, backlogged, and stale-snapshot states
- [ ] 9.14 Tests: each state renders distinctly and stale data is labelled with its age
- [ ] 9.15 Allowance authoring UI with the carve-out shown wherever the affected role is displayed
- [ ] 9.16 Tests: a role with an active subtractive allowance never renders as full access

## 10. Member surfaces

- [ ] 10.1 Self-service credential set and reset, scoped-to-infrastructure copy, existence and last-change status only
- [ ] 10.2 Tests: the credential value is never returned to the client or persisted; status renders from metadata alone
- [ ] 10.3 Connection instructions showing the add-on-reported account name and only the resources current entitlements reach
- [ ] 10.4 Tests: instructions change with entitlements and never list an unreachable resource

## 11. Retire the LLDAP bridge

- [ ] 11.1 Delete `sync/`, `backend/internal/services/lldap.go`, the `go-ldap/v3` dependency, and the sync Compose profile
- [ ] 11.2 Remove the LLDAP group-flattening convention and its tests; confirm no caller remains
- [ ] 11.3 Reduce the shadow vault to existence and rotation metadata; drop hash storage and its handlers
- [ ] 11.4 Tests: no hash column is written; existing status endpoints still answer from metadata
- [ ] 11.5 Remove LLDAP-specific environment variables from `.env.example`, `DEPLOY.md`, and `docker-compose.yml`

## 12. Documentation and graph refresh

- [ ] 12.1 Update `openspec/changes/syndra-core-architecture/design.md` §3 Bridge Plane and §9 interaction matrix for the add-on model
- [ ] 12.2 Update `ROADMAP.md` Phase 4 to record the LLDAP path as abandoned and the add-on platform as its replacement
- [ ] 12.3 Update `openspec/INDEX.md`, `openspec/NEXT.md`, and `specs/feature-coverage.md`
- [ ] 12.4 Update root `CLAUDE.md` and `README.md` for the removal of `sync/` and the addition of `addons/`
- [ ] 12.5 Run `detect_changes` and re-index the affected scope in codebase memory; record an ADR for the adapter-not-controller decision

## 13. P1 fixes

- [ ] 13.1 Reserved for post-review corrections

## 14. P2 fixes

- [ ] 14.1 Reserved for post-review corrections
