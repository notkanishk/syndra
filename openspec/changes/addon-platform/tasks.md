## 1. Schema and the target dimension

- [x] 1.1 Migration: `targets` registry table with `state` (`active | disabled`), seeded with `zitadel`
- [x] 1.2 Migration: rename `pending_zitadel_propagations` to a target-neutral name; the old name becomes false the moment a second target exists — it is `propagation_outbox`, and its indexes and CHECK constraints are renamed with it, since Postgres carries none of them across a table rename
- [x] 1.3 Migration: relax the Zitadel-shaped columns on the renamed outbox — `project_id`, `role_keys`, `zitadel_grant_id` become nullable and stay populated for Zitadel rows, enforced by a `target <> 'zitadel' OR (…)` CHECK rather than by convention — widen `op_type` to include `apply`, add `target` with an FK to `targets`, and index `(target, status, created_at)`. `status` gains `superseded` in the same migration so the drain needs no second ALTER (task 2.45)
- [x] 1.4 Migration: `desired_state_snapshots` — immutable rows per `(subject, target)` with a monotonic version; add-on rows carry their intent here rather than in the outbox's Zitadel columns. Immutability is a `BEFORE UPDATE OR DELETE` trigger and monotonicity a `BEFORE INSERT` one that **allocates** `MAX(version)+1` under a pair-scoped advisory lock, not conventions — `UNIQUE` alone permits version 2 followed by version 1, which makes "older than the last version applied" compare against a number that went backwards. Allocating rather than validating is what keeps a read-propose-retry loop out of every writer: the version is not a value a writer supplies, and `version` defaults so an INSERT can omit it
- [x] 1.5 Migration: plan storage — a plan row with id and bounded lifetime, plus one row per affected subject holding that subject's snapshot reference and the fingerprint of the reviewed state; the outbox references the per-subject plan row, not the snapshot directly, so one object holds what was approved, against what state, by whom. `plans.surface` records which rehearsal issued it, so a drift-triage plan cannot be cited on the bulk-grant apply endpoint; `plans_lifetime_check` binds `provisional` to a NULL `expires_at` and a non-NULL `state_read_at`
- [x] 1.6 Migration-coherence guard: plan expiry never deletes snapshots, which are audit records and outlive it; the outbox foreign key resolves to a plan subject row; no column on either table can hold a declared secret. No FK in the chain carries `ON DELETE CASCADE` or `SET NULL`, so a delete anywhere in it is refused rather than propagated
- [x] 1.7 Migration: add `target` to `drift_items` including its `drift_type` CHECK values (`zitadel_only` becomes `target_only`, moved as 000025 moved the rename before it) and the `idx_drift_items_pending_unique` index (rebuilt `NULLS NOT DISTINCT` so add-on rows, whose Zitadel columns are NULL, still dedupe), and to `external_grant_exclusions`' primary key, so two targets drifting on one user cannot suppress each other. Both sides of that key move with it: the mark-external `ON CONFLICT` names the widened key and takes the target from the drift row it claims, and `GetExclusions` reads scoped to a target rather than across all of them
- [x] 1.8 Migration-coherence guard tests: registry and state, rename, relaxed nullability, widened `op_type`, snapshot immutability and monotonic versions, drift CHECK and unique index include target, exclusions PK includes target, pre-existing rows read back as `zitadel`, and a row naming an unregistered target is refused — plus a guard that no live query still names the pre-rename table, and one that the down migration refuses to reinterpret non-`zitadel` rows rather than silently converting them into Zitadel ones
- [x] 1.9 Confirm `direct_role_grants` needs **no** target column and that nothing in this change reads or writes a non-`zitadel` direct grant
- [ ] 1.10 Thread `target` through the propagation claim, drain, and terminal-state writes in `services/propagation`; the drain claims one target per pass and skips targets whose registry state is not `active`
- [ ] 1.11 Tests: a drain for one target leaves other targets' rows untouched; a disabled target is skipped without erroring; an unreachable target halts only its own pass
- [ ] 1.12 Thread `target` through `services/drift` sweep and `drift_triage.go` so drift is classified per target
- [ ] 1.13 Tests: drift on one target does not appear under another; existing Zitadel drift behaviour is unchanged
- [ ] 1.14 Drift sweep consumes only reads marked current; a target served from a stale snapshot is recorded unreconciled rather than diffed
- [ ] 1.15 Tests: an outage produces no drift findings and does record an unreconciled target with the age of the last current read; reconciliation resumes on return
- [ ] 1.16 Thread `target` through `services/expiry` drain so allowance and grant expiry re-converge the right target
- [ ] 1.17 Tests: expiry sweep emits per-target propagations and leaves unrelated targets alone
- [ ] 1.18 Drift scoped to bound subjects: unbound target accounts are enumerated as unmanaged inventory, never entered into triage, and become managed only through the single adoption action
- [ ] 1.19 Tests: a first sweep against a target holding pre-existing unbound accounts raises no drift and reports them as inventory; adoption requires an operator decision
- [ ] 1.20 Per-`(subject, target)` serialization and stale-version rejection in the drain
- [ ] 1.21 Tests: an apply carrying an older version is rejected without dispatch; concurrent applies for one subject serialize; the settled state equals the higher version; a queued grant cannot land after a later revoke
- [ ] 1.22 Two snapshot read paths: operator-initiated applies dispatch the recorded snapshot, periodic reconcile resolves current state
- [ ] 1.23 Tests: a policy change between approval and drain still applies the approved snapshot; reconcile resolves fresh and never replays a superseded snapshot
- [ ] 1.24 Background revocation drain: a `periodic.Runner` alongside expiry and drift that claims only access-withdrawing rows, takes the same advisory lock as the operator drain, and never dispatches an access-conferring row
- [ ] 1.25 Tests: the runner drains revocations and leaves grants queued; it cannot run concurrently with an operator drain; a grant row is never claimed by it
- [ ] 1.26 Revocation priority within the operator drain: revocations dispatch before grants for the same target
- [ ] 1.27 Tests: a mixed queue dispatches revocations first; the retry budget applies per row without starving revocations

## 2. Add-on registry and wire contract

- [x] 2.1 Define the manifest types: entitlement schema, operation descriptors (`scope`, `confirm`, `secret_params`), target product and version. The manifest declares no target name — the registration owns it, so there is no mismatch case to resolve. Entitlement fields carry a `lifecycle` flag, the source structural mapping validation needs to reject them (task 7.4)
- [x] 2.2 Add-on registry: config-driven registration (base URL, client-certificate **or** signing-key reference — one transport mode is mandatory and a target with neither does not register at all), manifest fetch and cache, `deps.go` seam for tests. Registration and callability are separate states: `Init` contacts no add-on, so an unreachable one still registers (nav derives from deployment, not from what answers), and a refresh is what turns registration into capability. `Init` does reach the database: it upserts each configured target into the `targets` registry and disables the ones the deployment has dropped, because a target registered only in memory is one whose first snapshot, plan, outbox, or drift row the foreign key refuses. A refused refresh keeps the last accepted manifest rather than revoking a verified capability set
- [x] 2.3 Contract-version check at registration: refuse an add-on declaring a version the backend does not support, with that reason, so a mismatch fails at startup rather than as an absent field later. The version is declared by the manifest, not by backend configuration — a configured copy would only let the backend check itself against itself — so "at startup" is delivered by a synchronous, time-bounded first manifest read before the server accepts anything, not by a config field. It is loud but not fatal: refusing to boot the backend that governs every other target because one add-on shipped ahead of it would invert the fail-open rule. The refusal names both versions, because "the add-on is newer" and "the backend is newer" are different operator actions
- [x] 2.4 Tests: unregistered add-on is not callable; an operation absent from the manifest is rejected even if requested; registered-but-never-answered is a third distinct state rather than either of those
- [x] 2.5 Add-on HTTP client with per-add-on auth, timeouts, and a circuit breaker; carries plan id, fingerprints, and operation id on every mutating call — inside the signed body rather than beside it, so an intercepted call cannot be replayed against a different plan or subject. The callability gate runs before the network and the operation-record id is required before dispatch, so an uncallable operation costs nothing and a mutation cannot outrun its own audit trail. The breaker is per target and ignores deterministic refusals: a 4xx is a healthy add-on saying no, and letting one malformed request open the circuit would turn one operator's mistake into an outage for everyone
- [x] 2.6 Tests: client failure modes — unreachable, timeout, 5xx, open circuit — each map to the intended outcome and never to silent success. Four outcomes, not two, because "did it work" has three honest answers: **succeeded**, **rejected** (it refused and did not act), **unreached** (nothing happened, safe to retry), **indeterminate** (sent, answer lost, may have applied). Only unreached is retryable and indeterminate is neither a success nor a failure — retrying it duplicates a mutation and counting it either way asserts what the backend does not know. A 2xx whose body was truncated is indeterminate too — including one merely too large, since `io.LimitReader` signals its bound with EOF rather than an error and reading exactly the bound cannot tell a body that ended from one that was cut off. An oversized refusal keeps its status-derived outcome and records the truncation: reclassifying a deterministic refusal would turn it into a row that never settles. An already-cancelled context dispatches nothing
- [x] 2.7 Redaction layer keyed off `secret_params`: values stripped before any audit write, log line, or outbox payload. The secret set is resolved from the effective operation — policy ∩ manifest — never taken from the caller, because a caller that forgot to list its secret parameter is exactly the caller whose secret would otherwise be written. It fails closed: an operation that cannot be resolved has every value redacted, not none. `CallRequest` implements `String` and `GoString` so `%v` and `%#v` — the two verbs a future caller reaches for without thinking — cannot print a password; a redaction that depends on every caller remembering to redact is not one
- [x] 2.8 Tests: a secret parameter value never appears in emitted logs or any rendering of the request, asserted by scanning. Covers `String`/`%v`/`%s`/`%#v`/`%+v`, the transport's own failure log, the returned error, nested values, and a manifest that omits a policy-declared secret (policy prevails). Redaction copies rather than mutates, or it would strip the value the caller is about to send. Audit- and outbox-row scanning lands with 2.10/2.12, when there are rows to scan
- [ ] 2.9 Enqueue path for entitlement changes: outbox row plus audit row in one transaction with `target` set, before any add-on call
- [ ] 2.10 Tests: transactional rollback leaves no partial state; no add-on call is issued before commit
- [x] 2.11 Migration: `addon_operations` record table — operation id, target FK, actor, subject, operation name, status, timestamps; no parameter values column, and no free-text or JSON column at all. Not "a column we agree not to write secrets into": a `failure_detail` or `response_body` is precisely where a future maintainer puts an add-on's error payload, and that payload is the likeliest place for a submitted password to be echoed back. Five statuses — `dispatching` plus the four dispatch outcomes — and `settled_at` is constrained to be present exactly when the status is terminal, in both directions
- [x] 2.12 Migration-coherence guard test for `addon_operations`, including that there is no column able to hold a secret parameter value. The column set is asserted as a **closed list** rather than by scanning for suspicious names, so adding one fails the build and has to be argued for. Also: the status CHECK and the Go constants are one vocabulary, the record is born non-terminal, the unresolved predicate is indexed, and the down migration refuses while any operation is unresolved — those rows are the only surviving evidence that a secret-bearing call may have applied and nobody knows whether it did
- [x] 2.13 Secret-bearing dispatch protocol in `internal/services/addonop`: resolve callability, validate parameters, commit the record and its audit row in one transaction with a non-terminal status, read the record back and verify it names this exact call, dispatch under the resulting token, write the terminal status. The token is what makes the ordering structural: `DispatchRecord` has an unexported field, so it cannot be constructed outside `internal/addons`, and the only constructor reads the row and checks its target, operation, and subject against the call. Previously the transport asked for a non-empty string, which any caller could satisfy with a generated UUID — a rule that held in the one path that followed it and nowhere else. The lookup is scoped to `dispatching`, so a settled record cannot authorise a second dispatch. Never enqueued, never retried — a retry needs the parameters, the parameters are the secret, and keeping the secret to enable the retry is the vault this design exists to avoid. A settle that fails leaves the row `dispatching` and the result says so: the call already happened, and failing to record its outcome does not un-happen it. Confirmation-required operations are refused here, because a confirmation only the frontend enforces is a suggestion
- [x] 2.14 Tests: the seams record the ORDER, since the whole content of this protocol is an ordering and a per-call assertion cannot see one. A failed record write dispatches nothing; a failed terminal write leaves the row non-terminal, reports it, and repeats neither the call nor the settle; the secret reaches the add-on and appears in no committed parameter, no log line, and no returned result. Every outcome maps to a status the CHECK accepts, asserted against literal expected values rather than against the map under test — reading the expectation from the implementation passes for any mapping, including one recording an indeterminate dispatch as success
- [x] 2.15 Unresolved-operation surface: `dispatching` rows past a grace period plus every `indeterminate` row, oldest first. The grace period is what stops the surface flickering — a call in progress is indistinguishable by status from one whose backend died, and only elapsed time separates them. Counts split three ways and unresolved belongs to neither of the others: counted as success it claims what nobody knows, counted as failure it tells a member to retry against a target that may already hold their new credential
- [x] 2.16 Tests: the summary query is asserted to filter on exactly the three disjoint sets, including that the failure count does not absorb unresolved rows via `status <> 'succeeded'`; and `Unresolved()` agrees with the indexed predicate for every status in the vocabulary, so the surface and the index can never disagree about which rows are open
- [ ] 2.17 Plan store handler: `POST /plan` writes the plan and its per-subject rows from group 1's schema — no second store, and no per-subject state held anywhere but that row
- [ ] 2.18 Tests: a plan persists and expires; an apply citing an unknown or expired plan id is rejected
- [ ] 2.19 Apply gate: reject any apply not citing a backend-issued plan id; re-verify every fingerprint against live target state before dispatch
- [ ] 2.20 Tests: a client-supplied plan is refused; a fingerprint mismatch fails the apply, mutates nothing, and names the subjects that moved
- [ ] 2.21 Plan store secret exclusion: `secret_params` values are excluded from plan persistence; the value rides the apply request and is discarded with it
- [ ] 2.22 Tests: scan plan rows, indexes, and caches for a submitted secret value and assert absence; the apply still succeeds with the value carried transiently
- [ ] 2.23 Provisional plans: when a target is unreachable, record the approved snapshot and issue a plan against last-known state, labelled with its age and marked provisional
- [ ] 2.24 Tests: an entitlement change is accepted while the target is unreachable and yields a provisional plan, presented as recorded and awaiting the target, never as applied
- [ ] 2.25 Provisional plans are exempt from the ordinary plan lifetime, gated by re-fingerprinting rather than elapsed time, so a long outage cannot discard an approved change
- [ ] 2.26 Provisional resolution on return: re-fingerprint against live state, dispatch under the ordinary drain rules on match, withhold and require fresh approval on mismatch
- [ ] 2.27 Tests: a provisional plan outliving the ordinary lifetime is still valid; matching fingerprints dispatch without re-approval; differing fingerprints withhold and surface what changed
- [ ] 2.28 Log-head anchoring: persist each add-on's reported log head digest and record count, flagging a decreased count or a non-extending head as truncation
- [ ] 2.29 Tests: a truncated add-on log is detected by the anchor even though its remaining chain verifies
- [ ] 2.30 Queued accounting: extend the existing `BulkSummary.Queued` semantics to add-on targets so unconfirmed rows never count as succeeded
- [ ] 2.31 Tests: an unreachable add-on yields queued rows, a reachable one yields succeeded rows, and the summary distinguishes them
- [x] 2.32 Backend operation policy: per-operation-id scope, confirmation requirement, and parameter schema, owned by the backend and independent of any manifest. A Go table, not configuration — a policy an operator can edit at runtime is a second manifest with a friendlier name. **Enforced, not declared**: unknown keys are rejected rather than dropped (a caller sending a key the backend does not know has a different idea of the contract, and discarding it silently makes both sides wrong without either finding out), required values must be present and non-blank, types are checked, and a parameter type the validator cannot check is refused rather than waved through. No refusal names a value — parameter names are contract, values may be secrets, and an error string is logged, returned, and captured in traces. Validated at the caller so an invalid request never becomes a durable record, and again in the transport so no path to an add-on can skip it
- [x] 2.33 Manifest intersection: effective operation set is manifest ∩ policy with policy prevailing; unknown operation ids fail closed. Expressed as one rule that holds on every dimension — **the effective operation is the more restrictive of the two** — so a later dimension cannot be added with the comparison inverted: admin beats member, required confirmation beats none, secret parameters union, unavailable beats available
- [x] 2.34 Tests: a manifest cannot widen scope, cannot drop a confirmation requirement, cannot introduce an unknown operation, and can narrow; an unrecognised scope resolves to admin rather than defaulting permissive; a manifest omitting a policy-declared secret parameter cannot thereby make it loggable
- [x] 2.35 mTLS between backend and add-ons with a private CA, or signed requests carrying timestamp and body hash where mTLS is impractical. **Both modes require an https base URL and a non-https target does not register**: a client's TLS settings are consulted only when a handshake happens, so an `http://` URL means the certificate is never presented and the CA never consulted while the registration reports `auth=mtls` — the same wrong mental model an incomplete triple creates, reached through a URL scheme. Signed mode runs over TLS for the two properties a signature does not provide: an unencrypted secret-bearing body is readable on the internal network, and an unauthenticated response lets an on-path peer forge a 2xx the backend records as a completed mutation. A private CA configured alongside a signing key is that mode's anchor, not incomplete mTLS, and draws no warning. Redirects are refused rather than followed: Go re-sends the body on 307/308 and does not strip a custom header across hosts, so following one replays the secret-bearing call to an add-on-chosen host with its signature attached, and the redirect's own 2xx would be recorded against a target that never acted. HMAC-SHA256 over `"<unix>.<body>"`, the same construction the Zitadel Actions leg already verifies, so the system has one signature algorithm rather than two. TLS 1.3 minimum: both ends are ours and ship together, so there is no legacy peer to accommodate. The **manifest read goes over the same transport**, because a capability set read unauthenticated is one an on-path attacker can edit, and capability is what the backend then decides against
- [x] 2.36 Tests: a call without a valid client certificate is refused; a signature not matching body or timestamp is refused; a bare shared secret is insufficient; a non-https base URL does not register; signed mode refuses a server the configured private CA did not issue; a 302/307/308 never reaches a second server and never carries the body there, on the capability read as well as on a mutating call. Real certificates and a real handshake against a throwaway private CA — a mocked TLS layer would only prove the mock refuses. The signature verifier in the test is written independently of `ComputeSignature`, since a verifier built from the producer proves the producer agrees with itself, which was never in doubt
- [x] 2.37 Certificate and key material provisioning, rotation, and expiry surfacing for the add-on transport. Rotation is a file replacement detected by modification time, not a restart — bouncing the backend that governs every other target to rotate one target's certificate makes rotation an outage. A reload that fails keeps the last working material rather than failing every call in the window a rotation script leaves the file half written. Expiry reports the soonest date in the chain, not the client certificate alone: a current certificate against an expired CA fails exactly as hard as an expired one. Signing keys have their trailing newline trimmed (a mounted secret almost always carries one, and a one-byte difference surfaces as "no matching signature") and an empty key file is refused rather than authenticating everything under the empty string
- [x] 2.38 Tests: an expiring transport credential is surfaced before it fails; rotation does not drop in-flight operations — a call holding a client finishes on the material it started with, which for a secret-bearing dispatch is the difference between a completed operation and an indeterminate one nobody can resolve. Also: unchanged material is not re-parsed per call, unreadable material keeps the last good credential, missing material with no predecessor is an error rather than a silent plain client, signed mode reports no expiry rather than an unchecked "ok", and unregistering a target drops its loaded private key from memory
- [ ] 2.39 Cohort guard at plan time: the backend computes the affected-subject count and refuses to issue a plan exceeding the configured limit without an explicit scope acknowledgement
- [ ] 2.40 Tests: an oversized cohort is refused before any add-on is called and reports the computed count
- [ ] 2.41 Lifecycle-state refusals account as queued, never failed, and resume when the add-on returns to `active`
- [ ] 2.42 Tests: a `draining` or `read_only` refusal leaves the row queued, is excluded from failure counts, and resumes on return
- [ ] 2.43 Member-scope subject binding: reject any `member`-scoped operation whose subject is not the authenticated actor, enforced independently of manifest and policy
- [ ] 2.44 Tests: a member naming another subject is refused and no add-on call is made; a manifest declaring no subject constraint does not defeat the check
- [ ] 2.45 `superseded` as a terminal outbox state distinct from `failed`, for rows rejected on version
- [ ] 2.46 Tests: a grant overtaken by a later revoke terminates `superseded` and is excluded from failure counts and surfaces
- [ ] 2.47 Secret redaction across the transport and diagnostic layers: request logging, error responses, and panic captures on every leg including member-to-backend
- [ ] 2.48 Tests: a submitted secret appears in no logged request body, error payload, or captured trace
- [ ] 2.49 Rate-limit the member credential set per subject, refusing excess before any add-on call
- [ ] 2.50 Tests: the limit refuses excess without calling the add-on and is not reached by ordinary use
- [ ] 2.51 Background revocation runner hardening: escalate retry-budget exhaustion onto the unconfirmed-revocation surface as a finding, back off on lock contention, pre-flight target reachability
- [ ] 2.52 Tests: an exhausted revocation surfaces with its error rather than halting silently; lock contention backs off without spinning or starving; an unreachable target costs a probe, not a budget

## 3. Zitadel plan retrofit

- [ ] 3.1 Zitadel rehearsals write the same plan and per-subject rows, recording the intended outcome and a fingerprint of the reviewed state
- [ ] 3.2 Fingerprint the object that was reviewed, not only grants: the grant set for bulk operations, and the drift row's own status for triage, so a row someone else resolved fails verification
- [ ] 3.3 Tests: a plan persists with per-subject fingerprints and expires on its lifetime; a drift row resolved concurrently fails verification
- [ ] 3.4 Rehearsal responses on all four surfaces return a plan id — `POST /api/v1/grants/bulk`, `POST /api/v1/requests/bulk-decision`, `POST /api/v1/governance/drift/bulk-attribute`, `POST /api/v1/governance/drift/bulk-mark-external`
- [ ] 3.5 **BREAKING** Apply on those four surfaces cites the plan id instead of causing recomputation from a re-submitted request; `applyDriftPlan` and `applyBulkPlan` take the persisted plan, and fingerprints are re-verified before dispatch
- [ ] 3.6 Tests: an apply with no plan id is refused; an unknown or expired id is refused; a subject whose reviewed state moved between rehearsal and apply fails the apply, mutates nothing, and is named in the error
- [ ] 3.7 Confirm reconciliation stays read-only: `GET /api/v1/reconciliation/grants` has no apply path, and its rows reach mutation through drift triage
- [ ] 3.8 Update the frontend bulk, request-decision, and drift-triage flows to hold and send the plan id, with the stale-plan re-plan path
- [ ] 3.9 Tests: the UI sends only the plan id; a stale plan re-plans and shows which subjects moved rather than reporting a generic failure

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
- [ ] 4.14 Report the current log head digest and record count through `/health` for the backend to anchor
- [ ] 4.15 Tests: a record is durable before completion is reported; altering or removing a record breaks chain verification; a secret-bearing mutation logs the event without the value
- [ ] 4.16 Lifecycle state flag `active | draining | read_only`, settable without redeploy and reported through health
- [ ] 4.17 Tests: `read_only` refuses all mutations while serving reads; `draining` refuses new mutations while in-flight ones settle and reports when drained; neither is reported as unhealthy

## 5. TrueNAS add-on: entitlement plane

- [ ] 5.1 `POST /apply`: accept a resolved entitlement set for one subject, converge via `user.update({groups})`, level-triggered
- [ ] 5.2 `/apply` creates the account when absent as part of convergence and reports the derived name, so no separate creation operation has to be sequenced before it
- [ ] 5.3 Declare `enabled` and `smb_enabled` as entitlement-schema fields and converge them via `user.update({locked, smb})` on the same apply path
- [ ] 5.4 Tests: a set resolving both to disabled locks the account and clears SMB; a later set resolving them to enabled restores it with no second account created; neither path uses a creation operation
- [ ] 5.5 Tests: re-applying an unchanged set is a no-op with no mutating call; a reduced set converges to exactly the remaining groups
- [ ] 5.6 Per-request subject cap: refuse a request affecting more subjects than the configured limit without an explicit scope acknowledgement — defence in depth only, since a per-subject call cannot see a cohort
- [ ] 5.7 Tests: an oversized request is refused and returns the count it computed; the authoritative cohort guard is asserted backend-side in group 2, not here
- [ ] 5.8 `POST /plan`: returns `BulkPlan`/`BulkOutcome`-shaped outcomes with `Detail` and `Consequence`, plus a per-subject target-state fingerprint, mutating nothing
- [ ] 5.9 Tests: planning issues no mutating call, returns the apply path's shape, and produces a fingerprint that changes when the subject's target state changes
- [ ] 5.10 Fingerprint re-verification on apply: refuse the call if any supplied fingerprint no longer matches live target state
- [ ] 5.11 Tests: a subject mutated out of band between plan and apply causes refusal with that subject named, and nothing is applied
- [ ] 5.12 Operation-id deduplication for `/apply` and `/op/{name}`, backed by the idempotency store; the path segment is the operation name, the dedup token is the operation id, and they are never the same value
- [ ] 5.13 Tests: replaying an operation id returns the original outcome without a second mutating call
- [ ] 5.14 Absence handling: a subject missing from the expected set is reported as drift and never deleted or locked
- [ ] 5.15 Tests: a missing subject produces a drift report and no mutation

## 6. TrueNAS add-on: operations

- [ ] 6.1 Account creation inside `/apply`: query-then-create with a deterministic username derivation, reporting the created name; no standalone creation operation exists
- [ ] 6.2 Tests: applying twice creates no duplicate account and reports the same name; a subject with no account is planned as absent and created on apply
- [ ] 6.3 Username derivation from the email localpart: lowercase, strip sub-addressing, replace characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/`, enforce a valid leading character, truncate to 32; collision suffix from a stable hash of the Zitadel user ID
- [ ] 6.4 Tests: derivation is deterministic and always pattern-valid; non-ASCII, sub-addressed, leading-dot, and over-length localparts normalize; a forced collision resolves reproducibly and never reuses another subject's name
- [ ] 6.5 Normalization fallbacks: a localpart that normalizes to nothing usable falls back to a name derived from the Zitadel user id; the collision suffix is reserved before truncation, never appended after
- [ ] 6.6 Tests: an all-invalid and an empty-after-normalization localpart both yield deterministic valid names; a name needing both truncation and a suffix stays within the limit and still disambiguates
- [ ] 6.7 Binding conflict handling: an unbound account already holding the derived name halts the operation and surfaces **the same adoption action** the unmanaged inventory offers — one decision, two entry points, never a second code path that fires mid-convergence
- [ ] 6.8 Tests: a colliding unbound account causes no create, adopt, or modify, and surfaces the existing account for decision; adopting from the conflict and adopting from the inventory reach the same code and leave identical state
- [ ] 6.9 Binding recovery in reconcile: an account whose name changed out of band beneath a recorded binding is reported as a rename, not as a missing account
- [ ] 6.10 Tests: an out-of-band rename reports against the existing binding and creates no replacement account
- [ ] 6.11 Record the derived name against the subject as the authoritative binding; a later email change MUST NOT rename an existing account
- [ ] 6.12 Tests: an email change after creation leaves the account name unchanged and the binding still resolves the subject
- [ ] 6.13 `password.set`: `user.update({password})`, forwarded and never persisted, declared with `secret_params`
- [ ] 6.14 Tests: the plaintext appears in no store, snapshot, or log; the mutation log records that a password was set
- [ ] 6.15 `password.rotate`: mint and apply a new credential without returning or retaining it; the credential half of a revocation
- [ ] 6.16 Tests: rotation persists, caches, and logs no value, and records that a rotation occurred with actor and time
- [ ] 6.17 Revocation composition: the operator action writes the disabling allowance and enqueues the rotation, with copy stating that established sessions end on reconnect
- [ ] 6.18 Tests: revocation produces both halves; the surface never presents it as immediate session termination
- [ ] 6.19 `account.purge`: plan discloses retained home data before apply; `user.delete` runs on a backend-held elevated credential injected into that call alone, since the add-on's own key excludes deletion and no operator handles a target credential
- [ ] 6.20 Tests: purge without confirmation refuses; a purge on the add-on's own key is refused by the target for want of privilege; the injected credential is not persisted, cached, or logged; no operator credential prompt exists; the plan reports retained data size
- [ ] 6.21 `activity.get`: `audit.query` with `service: "SMB"`, reporting shares with auditing disabled
- [ ] 6.22 Tests: an empty result names the unaudited shares rather than implying no activity
- [ ] 6.23 `health.get`: `system.info`, `alert.list`, `pool.query`, `service.query` composed into the operator health shape
- [ ] 6.24 Tests: health composes all four sources and degrades per-source rather than failing whole

## 7. Role-to-target mapping and lifecycle

- [ ] 7.1 Migration: `target_role_mappings` binding `(target, project_id, role_key)` to an entitlement field and value, versioned with actor and reason like bundle definitions
- [ ] 7.2 Migration-coherence guard test: uniqueness on `(target, project_id, role_key, field)`, version history retained, rollback target reachable
- [ ] 7.3 Mapping CRUD with split validation — backend checks the field is in the add-on's declared schema and the role exists; the add-on confirms the value resolves on its target
- [ ] 7.4 Tests: an undeclared field is rejected without calling the add-on; a lifecycle field is rejected as a mapping target; an unresolvable value is rejected after the add-on reports it; a duplicate binding is rejected
- [ ] 7.5 Mapping versioning and rollback reusing the bundle version machinery
- [ ] 7.6 Tests: an edit creates a new version with actor and time; rollback restores the prior mapping and re-resolves affected subjects
- [ ] 7.7 Role-derived resolution: derive the role half of a subject's entitlement set from the mappings and from nothing else
- [ ] 7.8 Tests: a role with no mapping contributes nothing; two mappings on one role contribute both fields; resolution is stable under repetition
- [ ] 7.9 Lifecycle trigger on the existing grant path: look up targets mapped to the changed role and resolve the lifecycle entitlement fields in both directions, so gaining a first mapped role enables (creating the account through the apply) and losing the last disables
- [ ] 7.10 Tests: gaining a first mapped role creates the account through the apply itself with no separate sequencing; last mapped role removal disables and never deletes; regaining restores without operator action; an unmapped role triggers nothing
- [ ] 7.11 Mapping edit and delete plan through the standard plan path, subject to the blast-radius guard
- [ ] 7.12 Tests: deleting a mapping held by many subjects plans across all of them and trips the blast-radius guard without an acknowledgement

## 8. Allowances

- [ ] 8.1 Migration: allowance table with subject, target, entitlement field, value, `direction`, actor, reason, `expires_at`, `review_date`
- [ ] 8.2 Migration-coherence guard test, including the CHECK that a subtractive allowance requires `expires_at IS NOT NULL OR review_date IS NOT NULL`
- [ ] 8.3 Resolver, subtractive arm only: combine role-derived entitlements with subtractive allowances, deny beating allow
- [ ] 8.4 Tests: a subtractive allowance removes access, deny beats allow, and the resolved set is stable under re-resolution
- [ ] 8.5 Reject a subtractive allowance carrying neither expiry nor review date, with an error offering both valid forms and naming role-grant revocation — not a mapping edit — as the per-person permanent path
- [ ] 8.6 Tests: neither-bound is rejected; expiry-only is accepted; review-date-only is accepted
- [ ] 8.7 Expiry sweep removes lapsed allowances and re-converges the subject, writing an audit entry
- [ ] 8.8 Tests: a lapsed subtractive allowance restores role-derived access and records the restoration
- [ ] 8.9 Review-date governance surfacing: an allowance whose review date has passed appears for decision and stays in force until decided
- [ ] 8.10 Tests: a passed review date surfaces without lifting the suspension
- [ ] 8.11 Extend the lineage builder in `services/views.go` with the allowance band, carrying actor and grant time
- [ ] 8.12 Tests: every entitlement attributes to a source role or a derivation rule, and every suppressed entitlement attributes to the allowance suppressing it with its actor and time — the additive attribution case arrives with the additive arm
- [ ] 8.13 Defer the additive arm: no additive resolver path, authoring, or lineage rendering ships in phase 1, since quota and path grants are phase-2 Open Questions and would be an abstraction with no implementation behind it
- [ ] 8.14 Tests: submitting an additive allowance is refused as not-yet-supported rather than silently accepted and ignored

## 9. Operator surfaces

- [ ] 9.1 Manifest-driven operation rendering: member and admin panels built from `scope`, with no add-on-specific frontend code
- [ ] 9.2 Tests: an operation removed from a manifest disappears from the UI without a frontend change
- [ ] 9.3 Plan-then-apply flow reusing the `rehearse* → apply*` pattern, carrying the backend-issued plan id rather than round-tripping the plan body
- [ ] 9.4 Tests: apply is unreachable until a rehearsal has issued a plan id; the apply request carries the id rather than the original submission; an apply with no id is refused
- [ ] 9.5 Stale-plan recovery UX: a rejected apply re-plans and shows which subjects moved since the operator reviewed it
- [ ] 9.6 Tests: a subject changed between plan and apply produces the stale-plan path with that subject named, not a generic failure
- [ ] 9.7 Mapping management UI: role-to-target bindings with version history, rollback, and the plan shown before any edit or delete lands
- [ ] 9.8 Tests: an edit affecting many subjects shows the full plan and refuses without the blast-radius acknowledgement; rollback restores the prior version
- [ ] 9.9 Unconfirmed-revocation surface beside drift triage, with age-threshold escalation to a security-finding presentation
- [ ] 9.10 Tests: queued revokes are counted and presented apart from queued grants; crossing the threshold changes the presentation
- [ ] 9.11 Dormant-account housekeeping view: reason, age, individual and bulk action, plan before apply
- [ ] 9.12 Tests: accounts held by an active role are excluded; bulk action plans before applying
- [ ] 9.13 Remove `System > Hardware sync` from `ui/src/lib/nav.ts` and its route; add a per-target System entry per registered add-on, derived from deployment configuration rather than from what the current operator can see
- [ ] 9.14 Tests: nav renders a target entry for each registered add-on regardless of that operator's data, and the LLDAP entry and route are gone
- [ ] 9.15 Align the new operator surfaces with `basic-advanced-ia`: Basic versus Advanced placement, structure that does not move in response to data
- [ ] 9.16 Extend `GET /api/v1/governance/indicators` with the unconfirmed-revocation count and wire the `NavLeaf` indicator key so the badge can carry it
- [ ] 9.17 Tests: the indicator appears when unconfirmed revocations exist and clears when they resolve
- [ ] 9.18 Apply-surface disclosure: every submitted operation states whether it drains automatically or waits for an operator resume
- [ ] 9.19 Tests: a revocation says it will drain on its own, a grant says it is queued until resumed, and neither requires the operator to infer it
- [ ] 9.20 Add-on health surface distinguishing unreachable, read-only, draining, backlogged, and stale-snapshot states
- [ ] 9.21 Tests: each state renders distinctly and stale data is labelled with its age
- [ ] 9.22 Allowance authoring UI supporting both bounded forms — an expiry, or no expiry with a mandatory review date — and rejecting a denial with neither
- [ ] 9.23 Carve-out rendering wherever that role appears for that subject: user view, project role-holder lists, filtered cohorts, bulk selection
- [ ] 9.24 Tests: a role with an active subtractive allowance never renders as full access; a role-holder list never counts a suspended subject as holding the listed access
- [ ] 9.25 Review-date surfacing: an indefinite suspension appears in governance once its review date passes and stays in force until decided
- [ ] 9.26 Tests: a passed review date surfaces the suspension without lifting it

## 10. Member surfaces

- [ ] 10.1 Add a third `MEMBER_NAV` leaf `NAS/Network Storage` in `ui/src/lib/nav.ts`, present for every member regardless of entitlement, and extend the member route allow-list to cover it
- [ ] 10.2 Tests: the leaf renders for a member with no infrastructure access and does not appear or vanish as mapped roles change
- [ ] 10.3 Content gating on entitlement: a member with no role mapped to any target sees an explanation, and no credential form or connection instructions render
- [ ] 10.4 Content gating on account existence: a member holding a mapped role whose account is not yet created sees the pending state, with the credential affordance still withheld
- [ ] 10.5 Tests: all three states render distinctly — no entitlement, entitlement without account, account present — and the credential form appears only in the third
- [ ] 10.6 Self-service credential set and reset, scoped-to-infrastructure copy, existence and last-change status only
- [ ] 10.7 Tests: the credential value is never returned to the client or persisted; status renders from metadata alone
- [ ] 10.8 Connection instructions showing the add-on-reported account name and only the resources current entitlements reach
- [ ] 10.9 Tests: instructions change with entitlements and never list an unreachable resource
- [ ] 10.10 A credential set fails closed and says so when the target is unreachable, refusing for lifecycle state, or the account disappeared between render and submission — the backstop for that race, not the path for members without access
- [ ] 10.11 Tests: each case returns an explicit failure, records nothing as queued, and tells the member to retry

## 11. Retire the LLDAP bridge

- [ ] 11.1 Delete `sync/` in full, the sync Compose profile and its `SYNC_*` / `LLDAP_*` environment, and the `go-ldap/v3` dependency
- [ ] 11.2 Delete the intent pipeline: `db/intents.go`, `handlers/intents.go`, `services/provisioning.go`, `services/lldap.go` and their tests, the `/api/v1/intents*` routes in `router.go`, and the `ProvisioningIntent` types in `models.go`
- [ ] 11.3 Remove intent emission from `handlers/webhook.go`, `handlers/profile.go`, and `services/expiry/sweep.go` (the removal-intent path), plus their `deps.go` seams in `handlers/`, `services/`, and `services/expiry/`
- [ ] 11.4 Tests: no caller of the removed intent functions remains; webhook and expiry paths still complete their remaining work; `go vet ./...` is clean
- [ ] 11.5 Migration: drop `provisioning_intents` and its `lldap_group` column
- [ ] 11.6 Migration: drop `credential_hash`, `algorithm`, and `salt_params` from `shadow_credentials`, keeping existence and rotation metadata; paired coherence guard asserting no column can hold a credential hash
- [ ] 11.7 Tests: the vault status endpoints answer from metadata alone; no hash is written or readable
- [ ] 11.8 Member re-enrolment cutover: because stored hashes cannot be converted, every enrolled member must set a new credential — surface the un-enrolled state in the member view and prepare the operator communication before this step ships
- [ ] 11.9 Tests: a member with pre-cutover metadata and no new credential renders as needing enrolment, not as enrolled
- [ ] 11.10 Remove LLDAP-specific variables from `.env.example`, `DEPLOY.md`, and `docker-compose.yml`

## 12. Documentation and graph refresh

- [ ] 12.1 Update `openspec/changes/syndra-core-architecture/design.md` §3 Bridge Plane and §9 interaction matrix for the add-on model
- [ ] 12.2 Update `ROADMAP.md` Phase 4 to record the LLDAP path as abandoned and the add-on platform as its replacement
- [ ] 12.3 Update `openspec/INDEX.md`: remove or supersede the LDAP Sync and Provisioning capability rows so INDEX stops advertising a bridge that no longer exists
- [ ] 12.4 Mark `changes/syndra-core-architecture/specs/{ldap-sync,provisioning}/spec.md` superseded, pointing at this change
- [ ] 12.5 Update `openspec/NEXT.md` and `specs/feature-coverage.md`
- [ ] 12.6 Update root `CLAUDE.md` and `README.md` for the removal of `sync/` and the addition of `addons/`
- [ ] 12.7 Run `detect_changes` and re-index the affected scope in codebase memory; record ADRs for adapter-not-controller and for narrowing the operator-only drain rule

## Sequencing

226 tasks across 12 working groups. Two dependencies decide the order everything else falls into.

**1.5 (plan storage) gates the back half of §2 and all of §3.** Merging the plan and the snapshot onto one per-subject row means plan storage is schema, not handler state — so 2.17 through 2.26 (plan handler, apply gate, secret exclusion, provisional plans) and the entire Zitadel retrofit wait on it. §2's first sixteen tasks — manifest types, registry, client, redaction, enqueue, `addon_operations` — do not.

**A first commit that stands alone is 1.1–1.7 plus their guards.** Schema only, no behaviour change, revertible: the registry, the outbox rename and reshape, snapshots, plan storage, the drift target dimension. Everything downstream assumes it, and nothing about it assumes anything downstream.

**Group sizes**, for planning rather than for pride: §1 27, §2 52, §3 9, §4 17, §5 15, §6 24, §7 12, §8 14, §9 26, §10 11, §11 10, §12 7.

**§11 goes last on purpose.** It deletes the LLDAP path, and the vault reduction inside it is the point of no return — once the hashes are dropped, every member re-enrols and returning to LLDAP means doing it again.

## 13. P1 fixes

- [ ] 13.1 Reserved for post-review corrections

## 14. P2 fixes

- [ ] 14.1 Reserved for post-review corrections
