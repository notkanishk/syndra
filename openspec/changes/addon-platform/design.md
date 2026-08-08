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

### 3. One target column, not parallel tables

`pending_zitadel_propagations`, `direct_role_grants`, and the drift tables gain `target TEXT NOT NULL DEFAULT 'zitadel'`. The drain loop and sweep gain a target filter. Existing rows keep working untouched.

The permitted values come from a `targets` table with a foreign key, not a CHECK constraint. A CHECK would make registering a later add-on a schema migration — configuration and schema would have to move together, and a config-only deployment could write rows the database refuses. The table makes registration a data fact the drain can join against, so a row for an unregistered target is rejected at the boundary rather than dispatched into nothing. Migration seeds `zitadel`.

*Alternative considered:* per-target tables and drains. Rejected — three copies of the same convergence logic, drifting apart.

### 4. Two planes: entitlement and operation

| Plane | Shape | Machinery | Examples |
|---|---|---|---|
| Entitlement | level-triggered desired state | outbox, drift sweep, expiry, cascade | group membership, quota, path grants |
| Operation | one-shot event | audit only, no drift | `password.set`, `account.lock`, `account.purge`, `activity.get`, `health.get` |

The rule: anything with a desired state goes in the first, anything that is an event goes in the second.

This is what keeps plaintext out of the audit trail. `password.set` carries a secret and is an event, so it never touches the outbox — a durable intent row would write the member's password into Postgres and retain it.

**Staying out of the outbox must not mean staying out of the record.** Every Syndra-mediated mutation leaves a trace before the call, and a secret-bearing operation is no exception. It writes an `addon_operations` row — operation id, target, actor, subject, operation name, `status='dispatched'` — committed before the dispatch, carrying no parameter values. The terminal status is written after the response. A crash between the two leaves the row `dispatched`, which is the honest state: the target may or may not have applied it, and the operator surface says exactly that.

That row is a record, not a queue. It is never automatically retried, because retrying requires the secret and Syndra does not have it. Recovery is the member re-submitting, which is safe because the add-on deduplicates on operation id and because setting the same credential twice converges anyway. The distinction matters: the outbox drives work, `addon_operations` only witnesses it.

Level-triggering is available almost everywhere in both targets: `user.update({groups: [...]})`, `filesystem.setacl`, and `pool.dataset.set_quota` are all full replaces, as is UniFi's `PUT /users/:id/access_policies`. Retry is therefore safe by construction and "revoke partial" is the same call as "grant partial" with a different desired set.

### 5. The manifest declares an entitlement schema and an operation set

```
GET  /capabilities   → entitlement_schema, operations, target product + version
GET  /subjects       → full state read, feeds the existing drift sweep
POST /plan           → compute the effect of a proposed change; mutates nothing
POST /apply          → apply resolved entitlement state for one subject
POST /op/{id}        → one-shot operation from the manifest
GET  /health         → reachability, target version, last reconcile
```

`/plan` returns per-subject outcomes plus, for each subject, a **state fingerprint** — a hash of that subject's current state on the target. Every mutating call carries the fingerprints from the plan it came from, and the add-on refuses the call if any no longer matches. See §8.

`entitlement_schema` names the fields the target understands (`group[]`, `quota_bytes`, `path_grant[]`). Syndra fills them; it never learns what `lab_makers` means to TrueNAS. `operations` carries `scope` (member/admin), `confirm`, and `secret_params` — the last being a hard instruction that those values are never persisted or logged.

### 6. Two-layer access model: role plus allowance

**Layer 1 — Zitadel role.** A `truenas` project in Zitadel whose roles Syndra manages. The role is the sky-high statement of what a person can reach, mapped by operator config to a pre-existing TrueNAS group. Membership follows the role, is drift-checked, and expires through existing machinery.

**Layer 2 — Syndra allowance.** An explicit per-user overlay: quota, a specific path, a specific restriction. Recorded in Syndra, never inferred, and rendered as a visually distinct third band beside the Source and Derived bands `services/views.go` already produces. "Why does this user have access to X" answers with exactly one of: the role gives it, a rule derived it, or someone explicitly granted it — with actor and timestamp.

**Subtractive allowances MUST carry an expiry.** A deny is a time-boxed suspension — a safety violation, unpaid dues — not policy. Permanent removal means fixing the role mapping. Without this rule a user can hold a role whose access they silently do not have, and the role stops being a truthful statement of access.

Syndra resolves role and allowance into the final entitlement set; the add-on translates that set into TrueNAS constructs. Syndra decides who and what, the add-on decides how.

**The mapping is a first-class versioned model, not deployment config.** `target_role_mappings` binds `(target, project_id, role_key)` to a value for a field the add-on's entitlement schema declares — `role_key='maker'` to `group='lab_makers'`. It carries the same versioning, rollback, and audit as bundles, because a mapping edit silently changes what every holder of that role can reach and needs the same change history a bundle edit gets. Without it the resolver has no source for the role-derived half of an entitlement set.

Validation is split. Syndra checks structure: the field exists in the add-on's declared schema, the role exists, no duplicate binding for one `(target, project_id, role_key, field)`. The add-on checks reference: that `lab_makers` actually resolves on the target. Syndra cannot do the second — it does not know what the value means — so mapping writes are validated through the add-on and rejected if it cannot confirm the referent.

**The lifecycle trigger is the mapping table, consulted on grant change.** The existing grant path already emits propagations; it gains a lookup of which targets the changed role is mapped to. Gaining a first mapped role for a target enqueues `account.ensure` before the entitlement apply. Losing the last mapped role for a target enqueues `account.lock` — never a delete (§12). Deletion of a mapping is itself a grant-affecting change and re-resolves every holder through the same path, which is exactly the case the plan and blast-radius guards exist to catch.

### 7. Fail-open, with the queue made loud

An unreachable add-on does not block the grant. Syndra records it, the outbox holds the propagation, the drain retries. `BulkSummary.Queued` already carries exactly this semantic — *"rows Syndra recorded but could not confirm upstream. Kept apart from Succeeded so the headline cannot round them into success."*

Queued **revokes** are not symmetric with queued grants: a delayed grant is an inconvenience, a delayed revoke is retained access. They get a dedicated surface beside drift triage, with an age threshold above which an unconfirmed revoke is presented as a live security finding rather than a pending task.

### 8. Dry-run on every operation

Plans return outcomes in `BulkPlan`/`BulkOutcome` shape, so the existing `rehearse* → apply*` pattern in `handlers/drift_rehearsal.go` and its UI render unchanged. `BulkOutcome.Consequence` is where the add-on states what the subject is left holding afterwards.

**A dry-run flag alone cannot enforce plan-then-apply, and this is where the existing pattern is weakest.** `applyDriftPlan` takes the plan back from the client, and `BulkOutcome.GrantIDs` exists so the apply acts on identified rows "rather than re-guessing" — but nothing binds that returned plan to the one the backend computed. Between the two requests, entitlements can change, the target can change, and the cohort the operator reviewed can stop being the cohort that gets mutated. The operator approved a diff; something else applies.

So the plan is a backend-issued object, not a client round-trip:

- `POST /plan` computes the effect and persists it under a plan id with a short TTL, recording per-subject the resolved desired state and a fingerprint of the subject's current target state.
- Apply cites the plan id. The backend rejects an unknown or expired id, and re-verifies every fingerprint against live target state before dispatching.
- Any mismatch fails the apply with a distinct stale-plan error carrying the subjects that moved, so the surface can re-plan and show what changed rather than reporting a generic failure.

**This applies to Zitadel too, in this change.** The gap is not specific to add-ons — `applyDriftPlan` takes the plan back from the client with nothing binding it to what the backend computed, and the bulk and drift-triage paths share that shape. Leaving Zitadel on the weaker guarantee would mean two apply protocols, the older one being the one that governs production access today. The retrofit is mechanical because `BulkPlan`/`BulkOutcome` is already the shared vocabulary: the rehearse endpoints persist and return a plan id, the apply endpoints take the id instead of the body, and the fingerprint for a Zitadel subject is a hash of their current grant set. `BulkOutcome.GrantIDs` stays — it identifies rows within a plan — but it stops being the only thing standing between an approved diff and a different applied one.

**BREAKING:** the bulk, drift-triage, and reconciliation apply endpoints stop accepting a plan body. Clients send a plan id.

### 9. Add-on local state is a backstop, never a queue

| Store | Contents | Rebuildable |
|---|---|---|
| `idempotency` | key → result, 24h TTL; covers only `account.ensure` and `account.purge` | n/a |
| `snapshot` | last good target mirror | yes, from `user.query` |
| `mutations.log` | append-only JSONL of every write performed | no — that is the point |
| in-memory | ID cache: syndra id → uid, group name → gid | yes |

No command queue. Two durable queues would disagree about what is still pending, and Syndra's is the one that knows why the operation exists. The mutation log is not duplicated state — it is an independent forensic record that survives loss or tampering of Syndra's audit tables.

### 10. TrueNAS specifics

- **Use `user.update({password})`, never `user.set_password`.** The latter rejects the call unless the session is `FULL_ADMIN` when the target is another user; the former needs only `ACCOUNT_WRITE`. The add-on's TrueNAS identity is a dedicated local user in a group whose privilege grants `ACCOUNT_WRITE` and `SYSTEM_AUDIT_READ`, holding an API key with `expires_at` set.
- **Plaintext is mandatory.** No API accepts an NT or unix hash. Members set their own password in Syndra; it is forwarded and never stored.
- **Session revocation does not exist.** There is no `smb.status`, `smb.sessions`, or close/disconnect method. `account.lock` ships as `user.update({locked: true, smb: false})` plus a password rotation, and the UI MUST state that established sessions end on reconnect rather than immediately.
- **Quotas are storage, not bandwidth.** `pool.dataset.set_quota` / `get_quota`; Syndra owns the thresholds and alerting.
- **Hashes never leave the add-on.** `user.query` returns `unixhash` and `smbhash` — the NT hash is a pass-the-hash credential. The add-on passes `select` to fetch only what it needs and strips those fields on every path.
- **Activity needs enabling.** SMB auditing is per-share; `activity.get` reports which shares have it off rather than silently returning nothing.

### 11. Usernames derive from the email localpart, once, and are then recorded

The target generates nothing. No middleware method produces a username and `user.create` requires one. The webui carried a client-side generator wired to the full-name field — first initial plus last word, truncated at 8 characters, lowercased — which read the full name only, never the email, and resolved collisions by refusing to and making an operator rename the account by hand. Current master has removed it and now defaults `full_name` from the username instead.

The add-on therefore derives the name itself, from the Zitadel identity's primary email localpart: lowercase, sub-addressing stripped, characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` replaced, truncated to 32. Google Workspace is the sole IdP and already guarantees localpart uniqueness, so the collision suffix — a stable hash of the Zitadel user ID, never a counter — is a correctness backstop that should not fire in practice.

*Alternative considered:* reproducing the TrueNAS-native shape (`skhurana`) so Syndra-created accounts look like hand-created ones. Rejected because it derives from a mutable display name, collides readily across a shared surname, and still owes the collision resolution TrueNAS never wrote.

Two normalization edges need naming because both are silent-corruption bugs. A localpart that normalizes to nothing usable — all-invalid characters, or empty after stripping — falls back to a deterministic name derived from the Zitadel user id, never to a random or sequential one. And the collision suffix is reserved *before* truncation, not appended after: appending after truncating to 32 either overflows the limit or, if the truncation is redone, can produce a name that collides with the one it was meant to disambiguate.

**Derivation happens once, at account creation, and the resulting name is recorded.** Renaming a TrueNAS account disturbs its home directory, ACL entries, and SMB identity, so a later email change MUST NOT rename an existing account. This means the derivation is a recovery path for the common case, not a guarantee: if the add-on's store is lost, re-deriving recovers every subject whose email has not changed since creation, and the recorded binding in Syndra covers the rest. The recorded binding remains authoritative.

**Binding conflicts are an operator decision, never an inference.** `account.ensure` is query-then-create, which means it can find an account already holding the name it derived. Silently adopting it is the dangerous outcome — that account may belong to someone else entirely, and adopting it hands them a subject's entitlements. So an unbound account whose name collides is reported as a binding conflict and the operation stops. The operator chooses: adopt it, or create under a suffixed name. Reconcile detects the mirror case — a stable uid whose username changed underneath a recorded binding — and reports it the same way rather than treating it as a missing account to recreate.

### 12. Deprovisioning is reversible; purge is deliberate

Losing the last TrueNAS-granting role locks the account and clears `smb`, keeping the account and its home data. Purge stays a manual action behind the data checklist, driven from a dormant-account housekeeping view that lists stagnant accounts and supports individual and bulk action.

*Alternative considered:* auto-purge after a grace period. Rejected — automated deletion on critical infrastructure, triggered by a role change that may itself be the mistake.

### 13. The manifest is a ceiling, not a grant

An add-on declares what it *can* do. It does not decide what it is *allowed* to do. The backend holds its own policy for every operation id — the scope it may be offered at, whether it needs confirmation, its parameter schema — and the effective operation set is the intersection, with backend policy winning every disagreement. An operation id absent from backend policy is unavailable regardless of what the manifest says.

Without this the manifest is an authorization source, and a compromised or misconfigured add-on can declare `scope: "member"` on `account.purge` and have Syndra render it to members. The add-on is the least trusted component in the system — it holds the target credential and talks to a third-party API — so it must not be able to widen its own authority.

The cost is real and intended: adding an operation requires a backend policy entry, so a new add-on version cannot quietly grow its surface. Unknown operation ids fail closed.

### 14. Desired state is snapshotted, versioned, and applied in order per subject

An outbox row referencing "converge this subject" is under-specified: the drain runs later, the resolver recomputes, and what lands may not be what anyone approved. Each entitlement change therefore writes an immutable `desired_state_snapshots` row for `(subject, target)` with a monotonic version, and the outbox row references that snapshot.

**Two read paths, deliberately different.** An operator-initiated change is an approval and MUST apply its snapshot — the diff the operator saw is the diff that lands. The periodic reconcile is a convergence and MUST resolve current state instead, or it would spend forever replaying superseded snapshots and fighting every legitimate policy change. Conflating the two is how a reconcile loop reverts an intentional edit.

Application is serialized per `(subject, target)`, and an apply carrying a version older than the target's last applied version is rejected rather than executed. Otherwise a queued grant can land after a newer revoke and silently restore access. This hazard is not hypothetical here: the sync service carried `internal/worker/ordering.go` for exactly it, and deleting that service should not delete the lesson.

### 15. Add-on transport is mutually authenticated; the operation id is the replay token

Add-on calls use mTLS with a private CA, or signed requests carrying a timestamp and a body hash where mTLS is impractical. A bare shared secret is not sufficient: it authenticates the caller but binds nothing to the request, so an intercepted call can be replayed verbatim.

**No separate nonce store.** Every mutating call already carries an operation id the add-on deduplicates on, and every apply carries fingerprints re-verified against live target state, so a replayed mutation either hits the dedup store or fails verification. A nonce table would be a second replay-prevention mechanism guarding the same door, with its own expiry policy and its own failure mode. The operation id is the nonce, and the spec says so rather than building the machinery twice.

### 16. The mutation log has a durability contract

An append-only file with no stated guarantees is not a forensic record. `mutations.log` is written `0600`, fsynced per record — writes are rare enough at makerspace volume that batching buys nothing — and rotated by size with long retention, bounded only so the volume cannot fill.

Each record carries the digest of the record before it. A chain gives real tamper evidence for a few lines of code: entries cannot be altered or removed without breaking it, and verification is a single pass. Signing is deliberately skipped — the key would live on the same host as the log it protects, so it defends against almost nothing the chain does not, and it adds key management to a component that currently needs none.

Records are structured and redacted by the same `secret_params` rules as everything else.

### 17. Add-ons have a lifecycle state, and operations have individual availability

The read-only flag becomes a three-valued state: `active`, `draining` — refuse new mutations, let issued operations settle, keep serving reads — and `read_only`. Draining is what makes API-key rotation and target upgrades safe, and both will happen: the key carries `expires_at`, and TrueNAS majors break. All three are configuration, none requires a redeploy, and all three are visible to operators.

Version gating is per operation, not per major. A target major can be broadly supported while a specific method is absent — the research behind this design found methods moving across TrueNAS releases and per-feature floors throughout UniFi Access (user groups 2.2.6, webhooks 2.2.10, NFC import 3.3.10). The manifest therefore marks individual operations unavailable with a reason, and the operator surface shows them disabled and explained rather than absent or failing on use.

## Risks / Trade-offs

- **A queued revoke is retained access** → revokes drain ahead of grants, get a dedicated surface with age escalation (§7), and an unconfirmed revoke past threshold is a security finding, not a task. Containment when the add-on is unreachable is necessarily out of band — Syndra has no path to the target — so the escalation carries the manual procedure, and the drift sweep recognises the resulting change as reconciling rather than raising it as fresh drift. Otherwise doing the right thing under pressure generates an alert.
- **The add-on is the least trusted component and holds the target credential** → the manifest is a ceiling over backend policy, never an authorization source (§13); mTLS binds the transport (§15); the mutation log is tamper-evident (§16).
- **A stale queued change can undo a newer one** → snapshots are versioned, application is serialized per `(subject, target)`, and an older version is rejected rather than applied (§14).
- **The add-on's credential is broad** → `ACCOUNT_WRITE` plus `SYSTEM_AUDIT_READ` is most of what matters on the NAS. Scoped as tightly as the feature set allows, key expiry set and surfaced, read-only kill switch available without redeploy. Honest limit: scoping buys less once ACL and quota writes arrive in phase 2.
- **A mapping bug can revoke many people at once** → add-ons compute the diff and refuse operations exceeding a configured subject count unless the request carries an explicit scope acknowledgement. Combined with mandatory dry-run, this is the whole safety story for bulk effects.
- **Deletion-by-absence would be catastrophic** → tombstones only. A subject missing from a feed is logged as an anomaly and never actioned as a delete.
- **TrueNAS rate limits auth to 20 attempts per 60s with a 10-minute lockout** → one persistent WebSocket session, circuit breaker on the target so a lockout cannot wedge the drain.
- **TrueNAS API is versioned per release and breaks across majors** → add-on probes `system.version` at startup and refuses untested majors.
- **UniFi Access dies on Identity Enterprise** → the API is documented as unavailable after that upgrade (`CODE_OTHERS_UID_ADOPTED_NOT_SUPPORTED`). Known cliff, recorded before any work starts on that add-on.

## Migration Plan

1. Add `target` columns with `DEFAULT 'zitadel'`; existing rows and behaviour unchanged.
2. Add the add-on registry, manifest fetch, and contract, with no add-on registered. Backend behaviour identical.
3. Ship the TrueNAS add-on container behind an unregistered-by-default config, validated read-only first (`/subjects`, `/health`, `activity.get`).
4. Enable writes: account lifecycle, then member password.
5. Delete `sync/`, `services/lldap.go`, `go-ldap/v3`, and the LLDAP flattening convention. Reduce the shadow vault to existence and rotation metadata.

**Rollback:** steps 1–3 are additive and revert cleanly. After step 4, rollback means unregistering the add-on — TrueNAS accounts persist and are unmanaged until it returns, which is the same posture as an add-on outage under fail-open.

## Verification

```bash
cd backend && go test ./... && go vet ./...
cd addons/truenas && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
```

## Open Questions

- **Default quota by role.** Phase 2. Natural shape is a per-role default with a per-user allowance override, matching the two-layer model, but not yet decided.
- **Zitadel role granularity.** One `truenas` project is settled; whether per-lab-project access is expressed as distinct roles (`printing_member`) or as allowances over a single role is an operator modelling choice still open.
