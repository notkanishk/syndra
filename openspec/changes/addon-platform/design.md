# Add-on Platform Design

**Roadmap phase:** Phase 4, redefined ([ROADMAP](../syndra-core-architecture/ROADMAP.md)). Supersedes [sync-service](../sync-service/proposal.md) and reshapes [provisioning-intents](../provisioning-intents/proposal.md) and [shadow-password-vault](../shadow-password-vault/proposal.md).

## Context

Phase 4 built a bridge plane on LLDAP: the backend emits provisioning intents, a private Go worker claims them and reflects group membership and shadow passwords into an external LLDAP server. Group membership works. Password propagation was never validated — [shadow-password-vault/design.md](../shadow-password-vault/design.md) records this openly as a Research Conundrum, and the ROADMAP's LLDAP Integration item is paused on it.

The two systems that actually need provisioning both expose management APIs that make LDAP unnecessary:

- **TrueNAS SCALE** (lab storage, Samba): JSON-RPC 2.0 over WebSocket, versioned per release. REST was deprecated in 25.04 and removed in 26. An official Go client exists (`github.com/truenas/api_client_golang`). Auth is a user-linked API key via `auth.login_with_api_key`, with an expiry, and roles inherited from the linked user's group privileges.
- **UniFi Access** (doors): REST on `https://<console>:12445/api/v1/developer/`, `Authorization: Bearer`, self-signed cert. Assignment is declarative — `PUT /users/:id/access_policies` replaces the whole set and `[]` clears it.

Meanwhile Syndra already owns the machinery that a provisioning target needs: outbox-before-mutation (`pending_zitadel_propagations` + `services/propagation`), drift sweep and triage (`services/drift`, `services/drift_triage.go`), cascade (`services/cascade.go`), expiry drain (`services/expiry`), access lineage (`services/views.go`), and versioned policy with rollback. All of it is target-agnostic in everything but naming.

## Goals / Non-Goals

**Goals:**
- Reach TrueNAS SCALE and, later, UniFi Access directly, with no intermediate directory.
- Extend Syndra's existing IAM machinery over a target dimension rather than duplicating it per target.
- Keep the backend the single authority on entitlement; keep target mechanics entirely inside add-ons.
- Isolate each target's credentials and failure domain.
- Make every target-affecting operation dry-runnable before it applies.

**Non-Goals:**
- A plugin SDK, dynamic loading, or a capability-negotiation framework. Add-ons are Compose services registered in config.
- A second outbox, drift engine, or audit store inside any add-on.
- UniFi Access implementation. Its API is surveyed here so the contract does not have to change to accommodate it, but only the TrueNAS add-on ships in this change.
- Dataset and ACL lifecycle management. Phase 1 binds to TrueNAS groups created by hand.

## Decisions

### 1. Add-ons are target adapters, not autonomous controllers

An add-on is the analogue of `internal/zitadel/` — a driver behind Syndra's policy engine — not the analogue of `internal/services/`.

*Alternative considered:* an autonomous controller holding its own mapping policy, desired state, and reconcile loop, fed identity facts by Syndra. Rejected because it duplicates outbox, drift, audit, and versioning once per target, and because it breaks access lineage at the boundary: Syndra could say "we told it this person is a maker" but not "therefore they can write to `/mnt/lab/printing`". On a system where add-ons gate physical doors and lab storage, two-hop provenance is a real cost.

The heavy lifting an add-on does carry is target mechanics — username generation, group and ACL semantics, the WebSocket session and its rate limit, version probing, blast-radius validation. That is most of the code, and none of it is IAM logic.

### 2. Separate containers, internal network only

Each add-on runs as its own Compose service, reachable only by the backend, with no published host port.

*Alternative considered:* in-process Go packages behind an interface. Rejected on credential colocation — it would put the TrueNAS API key and the UniFi door token in the same process as the Zitadel service account, so one memory disclosure exposes identity, storage, and physical access together. A hung WebSocket would also take the backend down.

### 3. A target dimension on reshaped tables, not parallel ones

The propagation outbox and the drift tables gain `target TEXT NOT NULL DEFAULT 'zitadel'`, and the drain and sweep gain a target filter. Existing rows keep working untouched. That column alone is not enough, though — see below.

`direct_role_grants` does **not** get one. Direct grants are intents against Zitadel `user_grant`s; add-on entitlements come from mappings and allowances, which have their own tables. Nothing in this change reads or writes a non-`zitadel` direct grant, and a column no code path can populate is a column someone will later assume means something.

**Adding the column is not sufficient — both tables are Zitadel-shaped in their bones.** `pending_zitadel_propagations` requires `project_id VARCHAR(255) NOT NULL` and `role_keys TEXT[] NOT NULL`, carries `zitadel_grant_id`, and constrains `op_type` to `add | revoke | replace`. A TrueNAS entitlement apply has no project, no role keys, and no grant id. Rather than relaxing four constraints into meaninglessness, the target-specific payload moves wholly behind the `desired_state_snapshots` foreign key: the Zitadel columns become nullable and stay populated for Zitadel rows, `op_type` widens to include `apply`, and add-on rows carry their intent in the snapshot. The table also stops being called `pending_zitadel_propagations`, because that name becomes false the moment a second target exists — it becomes **`propagation_outbox`**, renamed in the same migration rather than left as a trap for the next reader.

Relaxing a `NOT NULL` is not the same as not caring about it. Each relaxed pair is paired with a CHECK — `target <> 'zitadel' OR (project_id IS NOT NULL AND role_keys IS NOT NULL)` — so what the add-on rows need does not become a licence to write a half-formed Zitadel row.

`drift_items` needs the same care and one thing more: `drift_type` is constrained to `zitadel_only | syndra_only`, and the pending-dedupe unique index on `(user_id, project_id, drift_type, role_keys)` would collide across targets — two targets drifting on the same user would silently suppress one. Target enters the CHECK values, the unique index, and `external_grant_exclusions`' primary key.

Entering the CHECK values means the type stops naming its target inside its own value: `zitadel_only` becomes **`target_only`** — "present on the target, unexplained by Syndra" — with the target named by the column beside it. `drift_type='zitadel_only'` on a TrueNAS row would be a false statement, and this is the cheapest moment the rename will ever be, since the constraint and the index are already open. It moves rows exactly as the MkAuth -> Syndra rename did in 000025. The rebuilt index also declares `NULLS NOT DISTINCT` (Postgres 15): an add-on row's `project_id` and `role_keys` are NULL, and under the default every re-detection would insert a fresh row and flood the queue that trust is set on the first day it fills.

The permitted target values come from a `targets` registry table with a foreign key, not a CHECK constraint. A CHECK would make registering a later add-on a schema migration — configuration and schema would have to move together, and a config-only deployment could write rows the database refuses.

That registry carries a `state` column (`active | disabled`), and **unregistering means disabling, never deleting**. The rollback plan says rollback is unregistration, and a foreign key would block deleting a row that still has propagation and drift history pointing at it — correctly, because that history must survive. "The drain must not dispatch work for an unregistered target" is therefore a state check the drain performs, not a property the foreign key provides; the key only guarantees the target was real.

*Alternative considered:* per-target tables and drains. Rejected — three copies of the same convergence logic, drifting apart.

### 4. Two planes: entitlement and operation

| Plane | Shape | Machinery | Examples |
|---|---|---|---|
| Entitlement | level-triggered desired state | outbox, drift sweep, expiry, cascade | group membership, quota, path grants, **account enabled, SMB enabled** |
| Operation | one-shot event | audit only, no drift | `password.set`, `password.rotate`, `account.purge`, `activity.get`, `health.get` |

The rule: anything with a desired state goes in the first, anything that is an event goes in the second.

**Lifecycle state is desired state, so it lives in the entitlement plane.** An earlier draft had `account.lock` as an operation, and that was an edge-triggered leak with a real consequence: deprovisioning left an account `locked` with SMB cleared, and regaining a mapped role could not bring it back, because a create-if-absent `ensure` sees an existing account and does nothing. The account would stay dark forever while Syndra believed access was restored.

So `enabled` and `smb_enabled` are fields in the entitlement schema, converged by `/apply` like any other field — but **resolver-computed, not mapping-bindable**. A mapping binding `role_key=X → enabled=false` would mean holding a role disables the account, colliding head-on with the derived lifecycle lock this decision exists to build, and the two rules would fight on every resolution. The resolver computes them from whether the subject holds any mapped role for the target; allowances may override them; structural mapping validation rejects them as mapping targets. Deprovisioning resolves them to false; regaining a mapped role resolves them to true and the same apply path restores the account. Nothing special-cases restoration because nothing special-cased suspension.

This also disambiguates two locks that would otherwise be indistinguishable on a target that has no field to tell them apart. A lifecycle lock is derived — the subject holds no mapped role — and clears itself when they do. An operator suspension is a subtractive allowance on `enabled`, carries an expiry like every subtractive allowance (§6), and survives re-resolution until it lapses or is lifted. An operator's deliberate suspension therefore cannot be undone by a role grant, and a lifecycle lock cannot outlive the condition that caused it.

**Account existence is desired state too, so `account.ensure` dissolves into the apply.** Keeping it as a separate operation created an ordering problem with no mechanism: the apply is a versioned snapshot on the outbox, `ensure` was a one-shot operation on another path, and nothing sequenced them — an apply could reach a subject whose account did not yet exist. Making existence part of convergence removes the ordering question rather than answering it. `/apply` creates the account when absent, reports the derived name in its outcome, and halts on a binding conflict (§11); a plan for a subject with no account fingerprints them as absent.

The lock button also stops being an operation: it is an allowance write plus `password.rotate` for the credential change, which is genuinely an event because it mints a new secret.

This is what keeps plaintext out of the audit trail. `password.set` carries a secret and is an event, so it never touches the outbox — a durable intent row would write the member's password into Postgres and retain it.

The absolute form of the trace rule — no target mutation without an outbox row first — therefore carries one stated exception, and only one: an operation declaring `secret_params` traces through `addon_operations` instead. Anything that can be queued is queued.

**Staying out of the outbox must not mean staying out of the record.** Every Syndra-mediated mutation leaves a trace before the call, and a secret-bearing operation is no exception. It writes an `addon_operations` row — operation id, target, actor, subject, operation name, `status='dispatched'` — committed before the dispatch, carrying no parameter values. The terminal status is written after the response. A crash between the two leaves the row `dispatched`, which is the honest state: the target may or may not have applied it, and the operator surface says exactly that.

That row is a record, not a queue. It is never automatically retried, because retrying requires the secret and Syndra does not have it. Recovery is the member re-submitting, which is safe because the add-on deduplicates on operation id and because setting the same credential twice converges anyway. The distinction matters: the outbox drives work, `addon_operations` only witnesses it.

**A credential set is rate-limited per member.** It is a write path a member can drive at will, and it terminates in a single rate-limited WebSocket session shared by every other operation the add-on performs. Repeated resets are a cheap way to wedge the target for everyone, deliberately or otherwise. The limit is per subject and generous enough that no honest member meets it.

**So a credential set fails closed.** It cannot be queued — queuing needs the secret, and not retaining it is the point — which means an unreachable target, a lifecycle-state refusal, or a subject whose account does not yet exist all produce an immediate, explicit failure telling the member to try again later. This is the one place the system does not fail open, and the member must be told plainly that nothing was recorded, because a member who believes their password was set and finds it was not will conclude the storage is broken rather than that they should retry.

Level-triggering is available almost everywhere in both targets: `user.update({groups: [...]})`, `filesystem.setacl`, and `pool.dataset.set_quota` are all full replaces, as is UniFi's `PUT /users/:id/access_policies`. Retry is therefore safe by construction and "revoke partial" is the same call as "grant partial" with a different desired set.

### 5. The manifest declares an entitlement schema and an operation set

```
GET  /capabilities   → entitlement_schema, operations, target product + version
GET  /subjects       → full state read, feeds the existing drift sweep
POST /plan           → compute the effect of a proposed change; mutates nothing
POST /apply          → apply resolved entitlement state for one subject
POST /op/{name}      → one-shot operation from the manifest
GET  /health         → reachability, target version, last reconcile
```

`/plan` returns per-subject outcomes plus, for each subject, a **state fingerprint** — a hash of that subject's current state on the target. Every mutating call carries the fingerprints from the plan it came from, and the add-on refuses the call if any no longer matches. See §8.

Plans persist intent, never secrets. A plan for a secret-bearing operation records that the operation will occur and against whom; the value rides the apply request and is discarded with it. The `secret_params` redaction rules cover the plan store exactly as they cover audit rows, outbox payloads, and logs — a plan is one more durable place a secret must never reach.

The manifest also declares the **contract version** it speaks — an integer, checked at registration, refused on mismatch. Add-ons are separately deployed containers, so one will eventually ship ahead of or behind the backend, and without a version that shows up as a field silently missing rather than a clean refusal at startup. One integer now is cheaper than a compatibility matrix later.

`entitlement_schema` names the fields the target understands (`group[]`, `quota_bytes`, `path_grant[]`). Syndra fills them; it never learns what `lab_makers` means to TrueNAS. `operations` carries `scope` (member/admin), `confirm`, and `secret_params` — the last being a hard instruction that those values are never persisted or logged.

### 6. Two-layer access model: role plus allowance

**Layer 1 — Zitadel role.** A `truenas` project in Zitadel whose roles Syndra manages. The role is the sky-high statement of what a person can reach, mapped by operator config to a pre-existing TrueNAS group. Membership follows the role, is drift-checked, and expires through existing machinery.

**Layer 2 — Syndra allowance.** An explicit per-user overlay: quota, a specific path, a specific restriction. Recorded in Syndra, never inferred, and rendered as a visually distinct third band beside the Source and Derived bands `services/views.go` already produces. "Why does this user have access to X" answers with exactly one of: the role gives it, a rule derived it, or someone explicitly granted it — with actor and timestamp.

**Phase 1 ships the subtractive half only.** The additive directions — quota, path grants — have no phase-1 consumer: both are Open Questions deferred to phase 2, so building the resolver arm, the authoring UI, and the lineage rendering for them now would be an abstraction with zero implementations behind it. The only thing phase 1 needs from this layer is operator suspension, which is subtractive.

The table keeps its `direction` column, because schema generality is the cheap part and migrating to add it later is not. What defers is the code: the additive resolver arm, additive authoring, and additive lineage rendering land with quotas, when a second consumer makes the abstraction real rather than anticipated.

**A subtractive allowance MUST be bounded in time, by an expiry or by a review date.** A deny is normally a time-boxed suspension — a safety violation, unpaid dues — and it expires on its own. But some suspensions are genuinely indefinite: an open incident, a safety ban with no agreed end. Those may omit the expiry only by carrying a mandatory review date, which surfaces in governance when it passes. What is forbidden is a denial with neither: an open-ended carve-out that nobody is ever prompted to revisit is how a temporary measure becomes permanent by inattention.

An earlier draft said permanent removal means fixing the role mapping. That was the wrong lever and worth correcting: a mapping edit changes access for *every* holder of that role, so using it to remove one person is a blast radius disguised as a policy fix. The per-person permanent path is revoking that person's role grant. The allowance layer exists for the case where the role should stay — because the reason, the actor, and the review date need to stay attached to the person rather than being erased into an absence.

The cost is that a subject can hold a role whose access they do not have, which is a trap unless it is visible. **So the carve-out renders wherever that role appears for that subject** — the user view, project role-holder lists, filtered cohorts, bulk selection. A role-holder list that shows someone as holding access they are suspended from is worse than not showing the list.

Syndra resolves role and allowance into the final entitlement set; the add-on translates that set into TrueNAS constructs. Syndra decides who and what, the add-on decides how.

**The mapping is a first-class versioned model, not deployment config.** `target_role_mappings` binds `(target, project_id, role_key)` to a value for a field the add-on's entitlement schema declares — `role_key='maker'` to `group='lab_makers'`. It carries the same versioning, rollback, and audit as bundles, because a mapping edit silently changes what every holder of that role can reach and needs the same change history a bundle edit gets. Without it the resolver has no source for the role-derived half of an entitlement set.

Validation is split. Syndra checks structure: the field exists in the add-on's declared schema, the role exists, no duplicate binding for one `(target, project_id, role_key, field)`. The add-on checks reference: that `lab_makers` actually resolves on the target. Syndra cannot do the second — it does not know what the value means — so mapping writes are validated through the add-on and rejected if it cannot confirm the referent.

**The lifecycle trigger is the mapping table, consulted on grant change.** The existing grant path already emits propagations; it gains a lookup of which targets the changed role is mapped to. Gaining a first mapped role resolves the lifecycle entitlement fields to enabled, which creates the account as part of convergence; losing the last resolves them to disabled — never a delete (§12), and never a bespoke restore path, because both directions are the same apply. Deletion of a mapping is itself a grant-affecting change and re-resolves every holder through the same path, which is exactly the case the plan and blast-radius guards exist to catch.

### 7. Fail-open, with the queue made loud

An unreachable add-on does not block the grant. Syndra records it and the outbox holds the propagation. `BulkSummary.Queued` already carries exactly this semantic — *"rows Syndra recorded but could not confirm upstream. Kept apart from Succeeded so the headline cannot round them into success."*

**There is no background drain today, and that is deliberate.** `POST /api/v1/propagations/drain` is the operator's explicit "Resume now", and the access-governance spec states it as a MUST: *"Buffered propagations MUST drain only on explicit operator action."* Only expiry and drift run periodically. Any design sentence that says "the drain retries" is therefore a claim about a worker that does not exist.

**This change adds exactly one background drain, for revocations only.** A delayed grant is an inconvenience; a delayed revoke is retained access, and leaving it to depend on someone opening the right page is the wrong dependency for the one case where time matters. Grants stay operator-gated on every target, preserving the consent property the existing MUST was written to protect — the property is about *conferring* access without a human, and a revocation confers nothing.

This requires a `MODIFIED Requirements` delta narrowing that MUST from "buffered propagations" to "buffered grants", with revocations exempted and the rationale recorded. It is a real weakening of a deliberate rule and it is scoped as narrowly as the goal allows.

**A background drain needs an exit the operator can see.** The inherited rule halts a drain once a row exceeds `OUTBOX_MAX_RETRIES`. In an operator-triggered drain that halt is loud because a human is watching it happen. In a background loop it is a silent permanent stop, on the one row class where delay *is* the security exposure. So a revocation exhausting its retry budget does not merely halt the pass: it escalates onto the unconfirmed-revocation surface as a finding, with its error, and stays there until resolved.

Two smaller properties follow. The runner backs off rather than spinning when it cannot take the advisory lock, so a busy operator drain slows it instead of starving it. And it pre-flights reachability per target, so an unreachable target costs one probe rather than a retry budget — the budget exists for real failures, not for a NAS that is switched off.

**Neither behaviour may be implicit.** The apply surface states, per operation, what will happen next: a revocation says it will drain on its own, a grant says it is queued until an operator resumes. An operator who has just approved something must never have to know which rule applied to infer whether anything more is required of them.

Queued revokes also get a dedicated surface beside drift triage, with an age threshold above which an unconfirmed one is presented as a live security finding rather than a pending task. With automatic draining that threshold now measures a target that is genuinely unreachable rather than a human who has not clicked.

### 8. Plan-then-apply becomes a backend guarantee

Plans return outcomes in `BulkPlan`/`BulkOutcome` shape, so the existing `rehearse* → apply*` pattern and its UI render unchanged. `BulkOutcome.Consequence` is where the add-on states what the subject is left holding afterwards.

**The existing pattern is weaker than it looks, but not in the way it first appears.** The plan never crosses the wire: `handleBulkAttributeDrift` calls `rehearseDriftBatch` and `applyDriftPlan` one line apart in the same handler, and `handleBulkGrants` re-rehearses server-side under `?apply=true`. There is no client-supplied plan body to reject, and no tampering vector there.

The real gap is between the two *requests*. An operator sends the rehearsal (`apply` absent), reads the consequences, and then sends a second request with `?apply=true` — which recomputes the plan from scratch. Nothing binds the second computation to the first. Grants, drift rows, mappings, and target state can all move in between, and the operator approves one diff while a freshly-computed and possibly different one applies. `BulkOutcome.GrantIDs` narrows this for two operations but is not a general answer.

So the rehearsal becomes a durable object rather than a throwaway computation:

- The rehearsal request persists its outcomes under a plan id with a bounded lifetime and returns the id alongside the plan. Each plan carries one row per affected subject holding that subject's desired-state snapshot and the fingerprint of the state that was reviewed, and the outbox row references **that** row rather than the snapshot directly.

  One approval, one durable object. An outbox row pointing at a snapshot while a plan separately held the fingerprints would have been two records of the same decision, free to disagree. With the snapshot and its fingerprint on one row, dispatch has exactly one thing to re-check. Plan expiry bounds how long an unexecuted plan may be cited; it MUST NOT delete the snapshots, which are audit records and outlive it.
- The apply request cites the plan id instead of re-submitting the original request and trusting recomputation. The backend rejects an unknown or expired id.
- **Fingerprints are re-verified at dispatch, not at apply.** Verifying at apply is the intuitive choice and it is wrong here: grants sit in the outbox until an operator resumes the drain, so the target can move between a verified apply and the actual write. "The diff you approved is the diff that lands" has to hold up to landing, not up to accepting. Add-on targets verify at dispatch already; Zitadel adopts the same point rather than inheriting a weaker one.
- Any mismatch fails the apply with a distinct stale-plan error carrying the subjects that moved, so the surface can re-plan and show what changed rather than reporting a generic failure.

**BREAKING:** the four rehearse-then-apply surfaces — `POST /api/v1/grants/bulk`, `POST /api/v1/requests/bulk-decision`, `POST /api/v1/governance/drift/bulk-attribute`, `POST /api/v1/governance/drift/bulk-mark-external` — stop treating `?apply=true` as "recompute and execute". Apply carries a plan id. Reconciliation is deliberately absent from that list: `GET /api/v1/reconciliation/grants` is a diff view with no apply endpoint, and its rows reach mutation through drift triage.

**This covers Zitadel in this change, not just add-ons.** Leaving Zitadel on the weaker guarantee would mean two apply protocols, with the looser one governing the access that actually exists today. `BulkPlan`/`BulkOutcome` is already the shared vocabulary, so the retrofit is mechanical.

A Zitadel fingerprint covers **the object the operator reviewed**, not only the subject's grants. For a bulk grant operation that is the grant set; for drift triage it must also include the drift row's own status, because `rehearseOneDrift` already handles the case where somebody resolved a row while the operator was reading the list. Fingerprinting grants alone would let exactly that case pass verification.

**A fingerprint needs a reachable target, and fail-open means the target may be unreachable.** These collide directly: §7 says an unreachable add-on must not block the entitlement decision, and a live fingerprint cannot be obtained from something that is not answering. Taken naively, an add-on outage would block operators from recording grants at all — the opposite of fail-open.

The resolution is that fail-open applies to the *entitlement decision*, never to unreviewed mutation. A change made while the target is unreachable is recorded, keeps its approved desired-state snapshot, and produces a **provisional plan**: computed against the add-on's last-known snapshot, fingerprinted from it, and labelled with that snapshot's age so nobody mistakes it for current truth.

**Provisional plans do not expire on the ordinary plan lifetime.** That lifetime exists to bound how long a *verified* plan may sit unexecuted; applying it to a provisional plan would silently discard approved changes whenever an outage outlasted it — turning a target outage into lost operator intent. A provisional plan lives until it resolves, and the re-fingerprint on return is its gate, not a clock.

When the target returns, the plan is re-fingerprinted against live state before anything dispatches:

- **Fingerprints match** — nothing moved while the target was away. The change proceeds under the ordinary drain rules (§7). The operator reviewed a diff that turned out to be accurate.
- **Fingerprints differ** — the target changed underneath the approval. The plan becomes stale, dispatch is withheld, and it requires fresh approval showing what moved.

That is the only defensible split. Dispatching into a changed world would apply a diff nobody saw, and demanding re-approval when nothing changed would punish operators for an outage they did not cause.

Provisional plans are visibly distinct from confirmed ones throughout, because "recorded and waiting" and "applied" are the distinction `BulkSummary.Queued` exists to protect.

**Scope of the planning requirement.** Every operator-initiated target-affecting action is planned — entitlement applies, mapping edits, and every admin-scoped operation. The one exemption is a `member`-scoped operation acting on the acting subject alone: a member setting their own credential has no cohort, no diff, and nothing to review, so it dispatches synchronously (§4). The exemption is by scope and subject, not by convenience, and it is the only one.

**Cost worth naming.** Fingerprint re-verification is a live per-subject read against the target, and the TrueNAS session is a single rate-limited WebSocket. At makerspace population this is a non-issue, but it is the kind of thing that gets batched wrong later — the read is per subject *within one plan*, not per subject per drain pass.

### 9. Add-on local state is a backstop, never a queue

| Store | Contents | Rebuildable |
|---|---|---|
| `idempotency` | operation id → result, for **every** mutating call | n/a |
| `snapshot` | last good target mirror | yes, from `user.query` |
| `mutations.log` | append-only JSONL of every write performed | no — that is the point |
| in-memory | ID cache: syndra id → uid, group name → gid | yes |

**Drift is computed only over bound subjects.** A real NAS holds accounts Syndra never provisioned — `root`, app service accounts, whatever an admin made by hand — and `/subjects` is a full state read. Diffing that against expected state would classify every one of them as untraced drift, and the first sweep after deployment would bury the queue in findings that are not findings. Trust in a triage queue is set on the day it first fills.

So drift covers subjects with a recorded binding. Unbound target accounts are an **unmanaged inventory**: enumerated, reported once so an operator can see what else lives on the target, and never entered into triage. An account moving from unmanaged to bound is an explicit adoption decision (§12), not something a sweep infers.

**Stale reads must not become drift.** The snapshot exists so an unreachable target can still answer a read, and the drift sweep consumes exactly those reads. Left unsaid, every outage would manufacture findings: the sweep would compare current desired state against an ageing mirror and report every intervening change as out-of-band. The sweep therefore consumes only reads the add-on marks current, and records the target as unreconciled for the duration instead — an absence of evidence, reported as such, rather than fabricated evidence of absence.

No command queue. Two durable queues would disagree about what is still pending, and Syndra's is the one that knows why the operation exists. The mutation log is not duplicated state — it is an independent forensic record that survives loss or tampering of Syndra's audit tables.

The idempotency store covers every mutating call, not a chosen few. §15 declines a nonce store on the grounds that the operation id already prevents replay, and that argument only holds if the dedup is universal. Its retention is therefore the actual replay window: a call replayed after the entry expires would be applied again. Two things bound that. Level-triggering makes re-application of an entitlement apply a no-op by construction. And in signed-request mode the signature timestamp rejects a stale request outright, independent of the store. The retention is sized to comfortably exceed any plausible retry or outage window rather than being tuned to the threat.

### 10. TrueNAS specifics

- **The add-on's privilege excludes `user.delete`.** Dataset and ACL lifecycle is an explicit non-goal, so phase 1 has no reason to grant deletion to a credential living in the least-trusted component. Purge is already manual, rare, and the one irreversible operation in the set, so it runs on a second elevated credential **held by the backend** and injected into that single call. The add-on never stores it, and a compromised add-on can misassign, disable, and rotate but cannot delete an account on its own.

  *Alternative considered:* having the operator supply the elevated credential at the moment of use, so no delete-capable secret exists at rest anywhere. Rejected on two grounds. It contradicts a rule this design states elsewhere — operators read target health *without* being granted target credentials — and it has no good source for the credential: either an operator types their personal TrueNAS admin password into a Syndra form, which is phishing training in reverse, or a shared elevated key lives in a password manager and gets pasted until it ends up in a browser. Backend-at-rest is the smaller cost. The backend is the mutation authority and already holds the Zitadel machine key, which can revoke every grant in the organisation; a TrueNAS delete key does not widen that blast radius. The threat being closed is add-on compromise, and this closes it without putting a privileged secret in a human's hands.
- **Use `user.update({password})`, never `user.set_password`.** The latter rejects the call unless the session is `FULL_ADMIN` when the target is another user; the former needs only `ACCOUNT_WRITE`. The add-on's TrueNAS identity is a dedicated local user in a group whose privilege grants `ACCOUNT_WRITE` and `SYSTEM_AUDIT_READ`, holding an API key with `expires_at` set.
- **Plaintext is mandatory.** No API accepts an NT or unix hash. Members set their own password in Syndra; it is forwarded and never stored.
- **Session revocation does not exist.** There is no `smb.status`, `smb.sessions`, or close/disconnect method. Revocation ships as an entitlement change resolving `enabled` and `smb_enabled` to disabled — `user.update({locked: true, smb: false})` — plus `password.rotate`, and the UI MUST state that established sessions end on reconnect rather than immediately.
- **Quotas are storage, not bandwidth.** `pool.dataset.set_quota` / `get_quota`; Syndra owns the thresholds and alerting.
- **Hashes never leave the add-on.** `user.query` returns `unixhash` and `smbhash` — the NT hash is a pass-the-hash credential. The add-on passes `select` to fetch only what it needs and strips those fields on every path.
- **Activity needs enabling.** SMB auditing is per-share; `activity.get` reports which shares have it off rather than silently returning nothing.

### 11. Usernames derive from the email localpart, once, and are then recorded

The target generates nothing. No middleware method produces a username and `user.create` requires one. The webui carried a client-side generator wired to the full-name field — first initial plus last word, truncated at 8 characters, lowercased — which read the full name only, never the email, and resolved collisions by refusing to and making an operator rename the account by hand. Current master has removed it and now defaults `full_name` from the username instead.

The add-on therefore derives the name itself, from the Zitadel identity's primary email localpart: lowercase, sub-addressing stripped, characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` replaced, truncated to 32. Google Workspace is the sole IdP and already guarantees localpart uniqueness **within one domain**, so the collision suffix — a stable hash of the Zitadel user ID, never a counter — is a correctness backstop that should not fire in practice. That "should not" rests entirely on the single-domain assumption: if Zitadel ever federates a second Workspace domain, two people can hold the same localpart and the suffix becomes routine rather than exceptional. It is built to be correct either way, but the claim that it stays dormant is conditional and stated here so nobody relies on it after the assumption changes.

*Alternative considered:* reproducing the TrueNAS-native shape (`skhurana`) so Syndra-created accounts look like hand-created ones. Rejected because it derives from a mutable display name, collides readily across a shared surname, and still owes the collision resolution TrueNAS never wrote.

Two normalization edges need naming because both are silent-corruption bugs. A localpart that normalizes to nothing usable — all-invalid characters, or empty after stripping — falls back to a deterministic name derived from the Zitadel user id, never to a random or sequential one. And the collision suffix is reserved *before* truncation, not appended after: appending after truncating to 32 either overflows the limit or, if the truncation is redone, can produce a name that collides with the one it was meant to disambiguate.

**Derivation happens once, at account creation, and the resulting name is recorded.** Renaming a TrueNAS account disturbs its home directory, ACL entries, and SMB identity, so a later email change MUST NOT rename an existing account. This means the derivation is a recovery path for the common case, not a guarantee: if the add-on's store is lost, re-deriving recovers every subject whose email has not changed since creation, and the recorded binding in Syndra covers the rest. The recorded binding remains authoritative.

**Binding conflicts are an operator decision, never an inference.** Account creation inside the apply is query-then-create, which means it can find an account already holding the name it derived. Silently adopting it is the dangerous outcome — that account may belong to someone else entirely, and adopting it hands them a subject's entitlements. So an unbound account whose name collides is reported as a binding conflict and the operation stops. The operator chooses: adopt it, or create under a suffixed name. Reconcile detects the mirror case — a stable uid whose username changed underneath a recorded binding — and reports it the same way rather than treating it as a missing account to recreate.

### 12. Deprovisioning is reversible; purge is deliberate

Losing the last TrueNAS-granting role resolves the lifecycle fields to disabled — `locked` set, `smb` cleared — keeping the account and its home data, and regaining a role resolves them back through the same apply (§4). Purge stays a manual action behind the data checklist, driven from a dormant-account housekeeping view that lists stagnant accounts and supports individual and bulk action.

*Alternative considered:* auto-purge after a grace period. Rejected — automated deletion on critical infrastructure, triggered by a role change that may itself be the mistake.

### 13. The manifest is a ceiling, not a grant

An add-on declares what it *can* do. It does not decide what it is *allowed* to do. The backend holds its own policy for every operation id — the scope it may be offered at, whether it needs confirmation, its parameter schema — and the effective operation set is the intersection, with backend policy winning every disagreement. An operation id absent from backend policy is unavailable regardless of what the manifest says.

Without this the manifest is an authorization source, and a compromised or misconfigured add-on can declare `scope: "member"` on `account.purge` and have Syndra render it to members. The add-on is the least trusted component in the system — it holds the target credential and talks to a third-party API — so it must not be able to widen its own authority.

The cost is real and intended: adding an operation requires a backend policy entry, so a new add-on version cannot quietly grow its surface. Unknown operation ids fail closed.

### 14. A `member` scope binds the subject, not just the audience

Scope decides who may invoke an operation. It must also decide *on whom*. A member-scoped operation MUST reject any subject other than the authenticated actor, enforced in the backend independently of both the manifest and the operation policy.

Without that, "scoped to `member`" only means "a member may call this", and `password.set` with somebody else's subject id resets their storage credential. The manifest cannot be trusted to bind it (§13), and the policy table describes operations rather than requests, so the check belongs at the request boundary and nowhere else.

### 15. Desired state is snapshotted, versioned, and applied in order per subject

An outbox row referencing "converge this subject" is under-specified: the drain runs later, the resolver recomputes, and what lands may not be what anyone approved. Each entitlement change therefore writes an immutable `desired_state_snapshots` row for `(subject, target)` with a monotonic version, and the outbox row references that snapshot.

**Two read paths, deliberately different.** An operator-initiated change is an approval and MUST apply its snapshot — the diff the operator saw is the diff that lands. The periodic reconcile is a convergence and MUST resolve current state instead, or it would spend forever replaying superseded snapshots and fighting every legitimate policy change. Conflating the two is how a reconcile loop reverts an intentional edit.

Application is serialized per `(subject, target)`, and an apply carrying a version older than the target's last applied version is rejected rather than executed.

**A version-rejected row terminates as `superseded`, not `failed`.** Revocation-first ordering makes this ordinary rather than exceptional: a grant at v5 queued behind a revoke at v6 is dispatched second and correctly refused. That is the system working, and labelling it `failed` shows the operator a phantom failure for a row deliberately discarded — on the row class where real failures matter most. Otherwise a queued grant can land after a newer revoke and silently restore access. This hazard is not hypothetical here: the sync service carried `internal/worker/ordering.go` for exactly it, and deleting that service should not delete the lesson.

### 16. Add-on transport is mutually authenticated; the operation id is the replay token

*(Naming: the path segment is the operation **name** — `POST /operations/password.set`, alongside `GET /capabilities` and `GET /health`. "Operation id" throughout this document means the per-call dedup token, never the operation's name. They were briefly the same word and that would have cost a debugging session, so the code refuses to reuse it: `CallRequest.Operation` is the name, `CallRequest.CallID` is the dedup token and the `addon_operations` row id, and neither field can be passed where the other is meant.)*

Add-on calls use mTLS with a private CA, or signed requests carrying a timestamp and a body hash where mTLS is impractical. A bare shared secret is not sufficient: it authenticates the caller but binds nothing to the request, so an intercepted call can be replayed verbatim.

**The base URL must be https, and that is not belt-and-braces over the above — it is what makes the above happen.** A client's TLS settings are consulted only where a handshake occurs, so an `http://` base URL means the client certificate is never presented and the private CA is never consulted, while the registration cheerfully reports `auth=mtls`. Signed mode needs it for the two properties a signature does not provide: over plaintext the secret-bearing body is readable by anything on the Compose network, and an on-path peer can forge a 2xx the backend records as a completed mutation — or forge a capability set. Signed mode therefore runs over TLS too, anchored on the private CA when one is configured (a CA alongside a signing key is that mode's anchor, not half-built mTLS) and on the system roots otherwise. There is no localhost exemption, because an exemption for development is an exemption that ships.

**The registered base URL is the only authority, and a redirect does not change it.** Go follows redirects by default and re-sends the body on 307 and 308; it strips `Authorization` and `Cookie` across hosts but not a custom header, so a mutating call answered with a redirect would replay the whole secret-bearing body to a host the add-on chose, signed and therefore authenticated to it — and in signed mode that host may verify against the system roots, which any publicly issued certificate satisfies. The redirect's own 2xx would then be recorded as success against a target that never acted. The client refuses redirects outright rather than following and re-checking one.

**The private CA is the mode, not an accessory to it.** Configure a client certificate and key without it and the backend verifies the add-on against the system root store — under which the add-on's own certificate fails and any publicly issued certificate passes. That is a different trust anchor, not a weaker one, so incomplete material is never treated as mutual TLS: it falls back to signed requests if a signing key is present, loudly, and refuses to register if not. An operator who believes mutual TLS is on while running signed requests has a wrong model of their own trust boundary and nothing else would tell them.

**Everything travels over that transport, including the capability read.** The manifest is what backend policy is intersected against, so an unauthenticated read of it hands an on-path attacker the capability set the backend then reasons from — withdraw a working operation, or offer one that must not be offered. The channel carrying the decision has to be as trustworthy as the one carrying the mutation.

**Four dispatch outcomes, not two.** `succeeded`, `rejected` (the add-on validated the call and refused; it did not act), `unreached` (nothing arrived), `indeterminate` (sent, answer lost). The split exists because a timeout and a connection refusal are not the same event: only `unreached` is safe to dispatch again, and `indeterminate` is neither success nor failure — auto-retrying it may duplicate a mutation the target already applied, and counting it either way asserts something the backend does not know. Ambiguous evidence resolves pessimistically, because being wrong towards `indeterminate` costs an operator a glance and being wrong towards `unreached` costs a duplicated mutation. `Call` returns no `error` alongside the outcome, deliberately: `err == nil` is a wrong proxy for success here, and a wrong proxy that reads like the right one is worse than none.

**The circuit breaker ignores rejections.** It is a health signal, and a 4xx is a healthy add-on saying no. Tripping on one would let a single operator's malformed request take a target offline for everybody; it trips on transport failure, 5xx, and 429 backpressure only. After the cooldown it simply allows again with its failure count intact, so one success clears it and the next failure re-opens it immediately — half-open behaviour without a half-open state to get wrong.

**No separate nonce store.** Every mutating call already carries an operation id the add-on deduplicates on, and every apply carries fingerprints re-verified against live target state, so a replayed mutation either hits the dedup store or fails verification. A nonce table would be a second replay-prevention mechanism guarding the same door, with its own expiry policy and its own failure mode. The operation id is the nonce, and the spec says so rather than building the machinery twice.

### 17. The mutation log has a durability contract

An append-only file with no stated guarantees is not a forensic record. `mutations.log` is written `0600`, fsynced per record — writes are rare enough at makerspace volume that batching buys nothing — and rotated by size with long retention, bounded only so the volume cannot fill.

Each record carries the digest of the record before it. A chain gives real tamper evidence for a few lines of code: an entry cannot be altered, and no entry can be removed from the middle, without breaking it.

**A chain alone does not detect tail truncation.** Delete the last N records and what remains verifies perfectly — the strongest attack against a local append-only log is also the simplest one, and a chain by itself cannot see it. So the head is anchored off the add-on's own volume: the add-on reports its current head digest and record count on `/health`, and Syndra persists each observation. A log whose count has gone backwards, or whose head does not extend the last one Syndra recorded, is truncation, and Syndra can say so.

This deliberately reuses the health path rather than introducing signed checkpoints or external object storage. The anchor's only job is to live somewhere the add-on cannot rewrite, and Syndra's database already is that — for a single-LXC deployment, adding object storage to hold one digest would be infrastructure serving a sentence of code. Signing is skipped for the same reason it was before: the key would live on the host it defends.

**Why the chain and anchor stay in phase 1.** It is fair to ask whether they earn their place when §2's motivating threat is a compromised add-on and neither defends against one. They stay because that is not the failure that actually happens. A rotation bug, a full disk, or a bad volume mount silently eating records is ordinary, and a decreasing record count against a persisted head is exactly what detects it. Fifteen lines and two tests buy detection of the realistic failure; the unrealistic one was never the claim.

**The limit of this, stated plainly.** The add-on is declared the least trusted component (§13), and it is also the thing reporting its own head digest and record count. A compromised add-on can truncate its log and report a consistent lie, and the anchor will agree with it. What the anchor actually detects is log loss, volume corruption, and tampering by anything that is not the add-on itself. That is worth having and it is not integrity against the add-on. The defences that do apply to a compromised add-on are elsewhere: it cannot widen its own authority (§13), it cannot exceed the backend's cohort limits (§15), and its mutations are independently visible in Syndra's own audit trail.

Records are structured and redacted by the same `secret_params` rules as everything else.

**Those rules cover the wire, not only the stores.** Listing tables, logs, and payloads leaves the paths a secret actually escapes through in practice: request-logging middleware that dumps bodies, error responses that echo the offending request, panic traces that capture arguments, and the member-to-backend leg before the value ever reaches a store. A declared secret parameter must be absent from all of them, and that is worth asserting in tests rather than assuming, because every one of those paths is code somebody adds later for debugging.

### 18. Add-ons have a lifecycle state, and operations have individual availability

The read-only flag becomes a three-valued state: `active`, `draining` — refuse new mutations, let issued operations settle, keep serving reads — and `read_only`. Draining is what makes API-key rotation and target upgrades safe, and both will happen: the key carries `expires_at`, and TrueNAS majors break. All three are configuration, none requires a redeploy, and all three are visible to operators.

**A lifecycle refusal accounts as queued, not failed.** A refusal is a terminal-looking response, and treating it as one would mean a deliberate maintenance window converted every pending revocation into a `failed` row — manufacturing exactly the false finality `BulkSummary.Queued` exists to prevent, and doing it during the window when an operator is least able to notice. Refusal for lifecycle state is indistinguishable in accounting from unreachability: the row stays queued and resumes when the state returns to `active`.

Version gating is per operation, not per major. A target major can be broadly supported while a specific method is absent — the research behind this design found methods moving across TrueNAS releases and per-feature floors throughout UniFi Access (user groups 2.2.6, webhooks 2.2.10, NFC import 3.3.10). The manifest therefore marks individual operations unavailable with a reason, and the operator surface shows them disabled and explained rather than absent or failing on use.

### 19. Navigation and the member surface

`System > Hardware sync` (`/system/hardware-sync`) is the LLDAP operator surface and is removed, replaced by a per-target entry under System for each registered add-on.

That sits close to `basic-advanced-ia`'s rule that structure never moves in response to data, so the distinction has to be explicit: **add-on registration is deployment configuration, not runtime data.** Nav derived from which add-ons a deployment runs is as stable as nav derived from which features a deployment has — it changes when someone deploys, not when someone's entitlements change. What the rule forbids is a nav row that appears because *this operator* has something to see there, and no per-target entry may work that way. An operator on a deployment with a TrueNAS add-on sees the TrueNAS entry whether or not it currently has drift.

Member add-on surfaces get their own `MEMBER_NAV` destination — a third leaf, **NAS/Network Storage**, beside `My access` and `Requests`. That name is target-specific on purpose: when UniFi Access lands, doors are a different thing in a different place and want their own leaf rather than a heading broad enough to swallow both. `My access` answers what a person is entitled to; this answers how they actually reach it, which is a different question with different content: a credential, a connection path, and later a PIN and a door list. Folding both under one heading would have worked for TrueNAS alone and stopped working the moment UniFi Access added doors.

**The leaf is always present.** Whether a member holds infrastructure access is exactly the kind of data `basic-advanced-ia` forbids structure from moving in response to — a nav row that appears when someone gains a role and vanishes when they lose it is the failure that rule exists to prevent. Audience decides structure; entitlement decides content.

**The content is gated, and gated on two things, not one.** A member with no mapped role for any target sees an explanation, not a form. A member who has the role but whose account has not been created yet — the grant is queued, or the target was unreachable when it landed — sees that state, and still no form. The credential affordance appears only once there is an account to set a credential on.

That gating is the design, not a convenience. A credential set cannot be queued, because queuing it would mean retaining the secret (§4), so offering the form before the account exists offers an action that can only fail. The fail-closed path stays as the backstop for the narrow race where an account disappears between render and submit; it is not the mechanism by which members without access are handled.

## Risks / Trade-offs

- **A queued revoke is retained access** → revokes drain ahead of grants, get a dedicated surface with age escalation (§7), and an unconfirmed revoke past threshold is a security finding, not a task. Containment when the add-on is unreachable is necessarily out of band — Syndra has no path to the target — so the escalation carries the manual procedure, and the drift sweep recognises the resulting change as reconciling rather than raising it as fresh drift. Otherwise doing the right thing under pressure generates an alert.
- **The add-on is the least trusted component and holds the target credential** → the manifest is a ceiling over backend policy, never an authorization source (§13); mTLS binds the transport (§16); the mutation log is tamper-evident (§17).
- **A stale queued change can undo a newer one** → snapshots are versioned, application is serialized per `(subject, target)`, and an older version is rejected rather than applied (§15).
- **The add-on's credential is broad** → `ACCOUNT_WRITE` plus `SYSTEM_AUDIT_READ` is most of what matters on the NAS. Scoped as tightly as the feature set allows, key expiry set and surfaced, read-only kill switch available without redeploy. Honest limit: scoping buys less once ACL and quota writes arrive in phase 2.
- **A mapping bug can revoke many people at once** → the cohort guard lives in the backend, because that is the only place the cohort exists. `/apply` is per subject, so an add-on sees one subject per call and can never compute "affected subject count" — specifying the guard there would have put it in the one component unable to implement it. The backend computes the cohort at plan time and refuses to issue a plan exceeding the configured subject count without an explicit scope acknowledgement. Add-ons keep a per-request cap as defence in depth against a backend that asks for too much at once. Combined with mandatory planning, this is the whole safety story for bulk effects.
- **Deletion-by-absence would be catastrophic** → tombstones only. A subject missing from a feed is logged as an anomaly and never actioned as a delete.
- **TrueNAS rate limits auth to 20 attempts per 60s with a 10-minute lockout** → one persistent WebSocket session, circuit breaker on the target so a lockout cannot wedge the drain.
- **TrueNAS API is versioned per release and breaks across majors** → add-on probes `system.version` at startup and refuses untested majors.
- **UniFi Access dies on Identity Enterprise** → the API is documented as unavailable after that upgrade (`CODE_OTHERS_UID_ADOPTED_NOT_SUPPORTED`). Known cliff, recorded before any work starts on that add-on.

## Migration Plan

1. Schema: `targets` registry with `state`, `desired_state_snapshots`, `addon_operations`, plan storage. Rename the propagation outbox, relax its Zitadel-shaped `NOT NULL`s, widen `op_type`, and add `target` to it and to the drift tables including their CHECK values and unique indexes. Existing rows and behaviour unchanged.
2. Retrofit plan-then-apply on the four Zitadel rehearse-then-apply surfaces. Backend and UI change together; no add-on involved.
3. Add the add-on registry, manifest fetch, and contract, with no add-on registered. Backend behaviour identical.
4. Ship the TrueNAS add-on container behind an unregistered-by-default config, validated read-only first (`/subjects`, `/health`, `activity.get`).
5. Enable writes: entitlement apply including account creation, then member credentials.
6. Retire the LLDAP path in full — `sync/`, the intent pipeline, the flattening convention, `go-ldap/v3` — and reduce the shadow vault.

**The vault reduction is a user-visible cutover, not a schema tidy-up.** `shadow_credentials.credential_hash` is `NOT NULL`, and every existing row holds an Argon2id hash that TrueNAS cannot accept in any form. There is no migration path for the credentials themselves: **every enrolled member must set a new password after cutover.** The hash column is dropped in a migration paired with a coherence guard like every other schema change here, the vault keeps existence and rotation metadata so drift still works, and the rollout needs the member communication planned before step 6 rather than discovered by members who find their storage stopped working.

**Rollback:** steps 1–4 are additive and revert cleanly *while Zitadel is still the only target*. Step 1's down migration refuses once any row names another one, rather than reinterpreting it: dropping the `target` column while a TrueNAS drift row exists turns it into a Zitadel drift row that never happened, sitting in an operator's triage queue. After step 5, rollback means setting the add-on's registry state to `disabled` — never deleting its row, which propagation and drift history still reference — and the down migration's refusal is what points an operator at that path instead of this one. TrueNAS accounts persist unmanaged until it returns, the same posture as an outage under fail-open. Step 6 is the point of no return: once the vault hashes are dropped, returning to LLDAP would require re-enrolling every member again.

## Verification

```bash
cd backend && go test ./... && go vet ./...
cd addons/truenas && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
test ! -d sync   # the bridge plane is gone, not merely unwired
```

## Open Questions

- **Default quota by role.** Phase 2. Natural shape is a per-role default with a per-user allowance override, matching the two-layer model, but not yet decided.
- **Zitadel role granularity.** One `truenas` project is settled; whether per-lab-project access is expressed as distinct roles (`printing_member`) or as allowances over a single role is an operator modelling choice still open.
