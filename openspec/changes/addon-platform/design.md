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

*Alternative considered:* per-target tables and drains. Rejected — three copies of the same convergence logic, drifting apart.

### 4. Two planes: entitlement and operation

| Plane | Shape | Machinery | Examples |
|---|---|---|---|
| Entitlement | level-triggered desired state | outbox, drift sweep, expiry, cascade | group membership, quota, path grants |
| Operation | one-shot event | audit only, no drift | `password.set`, `account.lock`, `account.purge`, `activity.get`, `health.get` |

The rule: anything with a desired state goes in the first, anything that is an event goes in the second.

This is what keeps plaintext out of the audit trail. `password.set` carries a secret and is an event, so it never touches the outbox — a durable intent row would write the member's password into Postgres and retain it.

Level-triggering is available almost everywhere in both targets: `user.update({groups: [...]})`, `filesystem.setacl`, and `pool.dataset.set_quota` are all full replaces, as is UniFi's `PUT /users/:id/access_policies`. Retry is therefore safe by construction and "revoke partial" is the same call as "grant partial" with a different desired set.

### 5. The manifest declares an entitlement schema and an operation set

```
GET  /capabilities   → entitlement_schema, operations, target product + version
GET  /subjects       → full state read, feeds the existing drift sweep
POST /apply          → apply resolved entitlement state for one subject; honours dry_run
POST /op/{id}        → one-shot operation from the manifest
GET  /health         → reachability, target version, last reconcile
```

`entitlement_schema` names the fields the target understands (`group[]`, `quota_bytes`, `path_grant[]`). Syndra fills them; it never learns what `lab_makers` means to TrueNAS. `operations` carries `scope` (member/admin), `confirm`, and `secret_params` — the last being a hard instruction that those values are never persisted or logged.

### 6. Two-layer access model: role plus allowance

**Layer 1 — Zitadel role.** A `truenas` project in Zitadel whose roles Syndra manages. The role is the sky-high statement of what a person can reach, mapped by operator config to a pre-existing TrueNAS group. Membership follows the role, is drift-checked, and expires through existing machinery.

**Layer 2 — Syndra allowance.** An explicit per-user overlay: quota, a specific path, a specific restriction. Recorded in Syndra, never inferred, and rendered as a visually distinct third band beside the Source and Derived bands `services/views.go` already produces. "Why does this user have access to X" answers with exactly one of: the role gives it, a rule derived it, or someone explicitly granted it — with actor and timestamp.

**Subtractive allowances MUST carry an expiry.** A deny is a time-boxed suspension — a safety violation, unpaid dues — not policy. Permanent removal means fixing the role mapping. Without this rule a user can hold a role whose access they silently do not have, and the role stops being a truthful statement of access.

Syndra resolves role and allowance into the final entitlement set; the add-on translates that set into TrueNAS constructs. Syndra decides who and what, the add-on decides how.

### 7. Fail-open, with the queue made loud

An unreachable add-on does not block the grant. Syndra records it, the outbox holds the propagation, the drain retries. `BulkSummary.Queued` already carries exactly this semantic — *"rows Syndra recorded but could not confirm upstream. Kept apart from Succeeded so the headline cannot round them into success."*

Queued **revokes** are not symmetric with queued grants: a delayed grant is an inconvenience, a delayed revoke is retained access. They get a dedicated surface beside drift triage, with an age threshold above which an unconfirmed revoke is presented as a live security finding rather than a pending task.

### 8. Dry-run on every operation

Both `/apply` and `/op/{id}` accept `dry_run` and return outcomes in `BulkPlan`/`BulkOutcome` shape, so the existing `rehearse* → apply*` pattern in `handlers/drift_rehearsal.go` and its UI render unchanged. `BulkOutcome.Consequence` is where the add-on states what the subject is left holding afterwards.

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

### 11. Deprovisioning is reversible; purge is deliberate

Losing the last TrueNAS-granting role locks the account and clears `smb`, keeping the account and its home data. Purge stays a manual action behind the data checklist, driven from a dormant-account housekeeping view that lists stagnant accounts and supports individual and bulk action.

*Alternative considered:* auto-purge after a grace period. Rejected — automated deletion on critical infrastructure, triggered by a role change that may itself be the mistake.

## Risks / Trade-offs

- **A queued revoke is retained access** → dedicated surface with age escalation (§7); an unconfirmed revoke past threshold is a security finding, not a task.
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

- **Username derivation.** The target generates nothing: no middleware method produces a username, and `user.create` requires one. The webui carried a client-side generator wired to the full-name field — first initial plus last word, truncated at 8 characters, lowercased — with no collision resolution beyond a validator that rejects and makes an operator fix it by hand. It derived from full name only, never email, and current master has removed it in favour of taking the username as input and defaulting `full_name` to it. The add-on must therefore derive the name itself, within `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` and 32 characters. Two candidates: the email localpart, whose uniqueness Google Workspace already guarantees so no collision logic is needed and the binding stays reconstructible from the Zitadel identity; or the TrueNAS-native shape, familiar to operators but derived from a mutable display name and requiring the collision suffix TrueNAS never solved. Leaning localpart.
- **Default quota by role.** Phase 2. Natural shape is a per-role default with a per-user allowance override, matching the two-layer model, but not yet decided.
- **Zitadel role granularity.** One `truenas` project is settled; whether per-lab-project access is expressed as distinct roles (`printing_member`) or as allowances over a single role is an operator modelling choice still open.
