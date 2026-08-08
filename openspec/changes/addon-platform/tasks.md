## 1. Target dimension in the data layer

- [ ] 1.1 Migration: add `target TEXT NOT NULL DEFAULT 'zitadel'` to `pending_zitadel_propagations`, `direct_role_grants`, and the drift tables, with a CHECK on the known target set and an index on `(target, status)`
- [ ] 1.2 Migration-coherence guard test asserting the new columns, defaults, CHECK, and index exist and that pre-existing rows read back as `zitadel`
- [ ] 1.3 Thread `target` through the propagation claim, drain, and terminal-state writes in `services/propagation`; the drain claims one target per pass
- [ ] 1.4 Tests: a drain for one target leaves other targets' rows untouched; an unreachable target halts only its own pass
- [ ] 1.5 Thread `target` through `services/drift` sweep and `drift_triage.go` so drift is classified per target
- [ ] 1.6 Tests: drift on one target does not appear under another; existing Zitadel drift behaviour is unchanged
- [ ] 1.7 Thread `target` through `services/expiry` drain so allowance and grant expiry re-converge the right target
- [ ] 1.8 Tests: expiry sweep emits per-target propagations and leaves unrelated targets alone

## 2. Add-on registry and wire contract

- [ ] 2.1 Define the manifest types: entitlement schema, operation descriptors (`scope`, `confirm`, `secret_params`), target product and version
- [ ] 2.2 Add-on registry: config-driven registration (base URL, shared secret ref), manifest fetch and cache, `deps.go` seam for tests
- [ ] 2.3 Tests: unregistered add-on is not callable; an operation absent from the manifest is rejected even if requested
- [ ] 2.4 Add-on HTTP client with per-add-on auth, timeouts, and a circuit breaker; `dry_run` supported on every call
- [ ] 2.5 Tests: client failure modes — unreachable, timeout, 5xx, open circuit — each map to the intended outcome and never to silent success
- [ ] 2.6 Redaction layer keyed off `secret_params`: values stripped before any audit write, log line, or outbox payload
- [ ] 2.7 Tests: a secret parameter value never appears in audit rows, outbox rows, or emitted logs, asserted by scanning the written records
- [ ] 2.8 Enqueue path for entitlement changes: outbox row plus audit row in one transaction with `target` set, before any add-on call
- [ ] 2.9 Tests: transactional rollback leaves no partial state; no add-on call is issued before commit
- [ ] 2.10 Reject secret-bearing operations from the outbox path; dispatch them synchronously with an event-shaped audit record
- [ ] 2.11 Tests: a secret-bearing operation creates no outbox row and does create an audit record without the value
- [ ] 2.12 Queued accounting: extend the existing `BulkSummary.Queued` semantics to add-on targets so unconfirmed rows never count as succeeded
- [ ] 2.13 Tests: an unreachable add-on yields queued rows, a reachable one yields succeeded rows, and the summary distinguishes them

## 3. TrueNAS add-on: skeleton and read path

- [ ] 3.1 New Go module `addons/truenas` with Dockerfile and Compose service, internal network only, no published port
- [ ] 3.2 TrueNAS client wrapper over `github.com/truenas/api_client_golang`: single persistent session, `auth.login_with_api_key`, reconnect
- [ ] 3.3 Tests: session is reused across calls; a rate-limit response opens the circuit instead of retrying
- [ ] 3.4 `GET /health`: reachability, `system.version` probe, key expiry, last successful read, read-only flag, circuit state
- [ ] 3.5 Tests: unsupported target major version refuses mutations and reports the supported range through health
- [ ] 3.6 `GET /capabilities`: manifest with entitlement schema and the phase-1 operation set
- [ ] 3.7 `GET /subjects`: state read via `user.query` and `group.query` with an explicit `select`, feeding the drift sweep
- [ ] 3.8 Tests: hash fields are absent from every response, snapshot, and log entry — asserted by scanning serialized output
- [ ] 3.9 Local stores: bbolt idempotency bucket with TTL, snapshot bucket, append-only JSONL mutation log; no command queue
- [ ] 3.10 Tests: snapshot serves a stale read with its age when the target is unreachable; the mutation log records every write
- [ ] 3.11 Read-only mode flag: refuses mutating operations, continues serving state and health
- [ ] 3.12 Tests: read-only mode refuses every mutating operation and is reported as read-only rather than failing

## 4. TrueNAS add-on: entitlement plane

- [ ] 4.1 `POST /apply`: accept a resolved entitlement set for one subject, converge via `user.update({groups})`, level-triggered
- [ ] 4.2 Tests: re-applying an unchanged set is a no-op with no mutating call; a reduced set converges to exactly the remaining groups
- [ ] 4.3 Blast-radius limiter: compute affected subject count, refuse beyond the configured limit without an explicit scope acknowledgement
- [ ] 4.4 Tests: an oversized effect is refused, returns the computed count, and mutates nothing
- [ ] 4.5 Dry-run: `/apply` returns `BulkPlan`/`BulkOutcome`-shaped outcomes with `Detail` and `Consequence`, mutating nothing
- [ ] 4.6 Tests: dry-run issues no mutating call and returns the same shape the apply path returns
- [ ] 4.7 Absence handling: a subject missing from the expected set is reported as drift and never deleted or locked
- [ ] 4.8 Tests: a missing subject produces a drift report and no mutation

## 5. TrueNAS add-on: operations

- [ ] 5.1 `account.ensure`: query-then-create with a deterministic username derivation, report the created account name back, idempotency-keyed
- [ ] 5.2 Tests: repeat invocation creates no duplicate account and returns the same account name
- [ ] 5.3 Username derivation: normalize to `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` within 32 characters, with a deterministic collision suffix as an unreachable fallback
- [ ] 5.4 Tests: non-ASCII and invalid characters normalize; an over-length source truncates stably; a forced collision resolves deterministically and never reuses another subject's name
- [ ] 5.5 `password.set`: `user.update({password})`, forwarded and never persisted, declared with `secret_params`
- [ ] 5.6 Tests: the plaintext appears in no store, snapshot, or log; the mutation log records that a password was set
- [ ] 5.7 `account.lock`: set `locked`, clear `smb`, rotate password; response states that established sessions end on reconnect
- [ ] 5.8 Tests: lock applies all three effects and the response carries the reconnect caveat
- [ ] 5.9 `account.smb.set`: reversible SMB suspend and resume
- [ ] 5.10 Tests: suspend then resume restores the prior state
- [ ] 5.11 `account.purge`: plan discloses retained home data before apply; `user.delete` only on explicit confirmation
- [ ] 5.12 Tests: purge without confirmation refuses; the plan reports retained data size
- [ ] 5.13 `activity.get`: `audit.query` with `service: "SMB"`, reporting shares with auditing disabled
- [ ] 5.14 Tests: an empty result names the unaudited shares rather than implying no activity
- [ ] 5.15 `health.get`: `system.info`, `alert.list`, `pool.query`, `service.query` composed into the operator health shape
- [ ] 5.16 Tests: health composes all four sources and degrades per-source rather than failing whole

## 6. Allowances

- [ ] 6.1 Migration: allowance table with subject, target, entitlement field, value, direction, actor, reason, `expires_at`
- [ ] 6.2 Migration-coherence guard test, including the CHECK that a subtractive allowance requires a non-null `expires_at`
- [ ] 6.3 Resolver: combine role-derived entitlements with additive and subtractive allowances into the resolved set handed to the add-on
- [ ] 6.4 Tests: additive extends, subtractive removes, deny beats allow, and the resolved set is stable under re-resolution
- [ ] 6.5 Reject subtractive allowances submitted without an expiry, with an error directing the operator to the role mapping
- [ ] 6.6 Tests: submission without expiry is rejected; with expiry is accepted
- [ ] 6.7 Expiry sweep removes lapsed allowances and re-converges the subject, writing an audit entry
- [ ] 6.8 Tests: a lapsed subtractive allowance restores role-derived access and records the restoration
- [ ] 6.9 Extend the lineage builder in `services/views.go` with the allowance band, carrying actor and grant time
- [ ] 6.10 Tests: every entitlement attributes to exactly one of source, derived, or allowance

## 7. Operator surfaces

- [ ] 7.1 Manifest-driven operation rendering: member and admin panels built from `scope`, with no add-on-specific frontend code
- [ ] 7.2 Tests: an operation removed from a manifest disappears from the UI without a frontend change
- [ ] 7.3 Plan-then-apply flow reusing the `rehearse* → apply*` pattern for every target-affecting operation
- [ ] 7.4 Tests: apply is unreachable until a plan has been presented; apply acts on the planned rows
- [ ] 7.5 Unconfirmed-revocation surface beside drift triage, with age-threshold escalation to a security-finding presentation
- [ ] 7.6 Tests: queued revokes are counted and presented apart from queued grants; crossing the threshold changes the presentation
- [ ] 7.7 Dormant-account housekeeping view: reason, age, individual and bulk action, plan before apply
- [ ] 7.8 Tests: accounts held by an active role are excluded; bulk action plans before applying
- [ ] 7.9 Add-on health surface distinguishing unreachable, read-only, backlogged, and stale-snapshot states
- [ ] 7.10 Tests: each state renders distinctly and stale data is labelled with its age
- [ ] 7.11 Allowance authoring UI with the carve-out shown wherever the affected role is displayed
- [ ] 7.12 Tests: a role with an active subtractive allowance never renders as full access

## 8. Member surfaces

- [ ] 8.1 Self-service credential set and reset, scoped-to-infrastructure copy, existence and last-change status only
- [ ] 8.2 Tests: the credential value is never returned to the client or persisted; status renders from metadata alone
- [ ] 8.3 Connection instructions showing the add-on-reported account name and only the resources current entitlements reach
- [ ] 8.4 Tests: instructions change with entitlements and never list an unreachable resource

## 9. Retire the LLDAP bridge

- [ ] 9.1 Delete `sync/`, `backend/internal/services/lldap.go`, the `go-ldap/v3` dependency, and the sync Compose profile
- [ ] 9.2 Remove the LLDAP group-flattening convention and its tests; confirm no caller remains
- [ ] 9.3 Reduce the shadow vault to existence and rotation metadata; drop hash storage and its handlers
- [ ] 9.4 Tests: no hash column is written; existing status endpoints still answer from metadata
- [ ] 9.5 Remove LLDAP-specific environment variables from `.env.example`, `DEPLOY.md`, and `docker-compose.yml`

## 10. Documentation and graph refresh

- [ ] 10.1 Update `openspec/changes/syndra-core-architecture/design.md` §3 Bridge Plane and §9 interaction matrix for the add-on model
- [ ] 10.2 Update `ROADMAP.md` Phase 4 to record the LLDAP path as abandoned and the add-on platform as its replacement
- [ ] 10.3 Update `openspec/INDEX.md`, `openspec/NEXT.md`, and `specs/feature-coverage.md`
- [ ] 10.4 Update root `CLAUDE.md` and `README.md` for the removal of `sync/` and the addition of `addons/`
- [ ] 10.5 Run `detect_changes` and re-index the affected scope in codebase memory; record an ADR for the adapter-not-controller decision

## 11. P1 fixes

- [ ] 11.1 Reserved for post-review corrections

## 12. P2 fixes

- [ ] 12.1 Reserved for post-review corrections
