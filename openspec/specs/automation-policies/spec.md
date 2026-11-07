# automation-policies Specification

## Purpose
TBD - created by archiving change wave-2-part-4-zitadel-state-projection-and-drift-control. Update Purpose after archive.
## Requirements
### Requirement: Bundle and mapping-rule changes MUST project their effective grants into Zitadel through the outbox

When an operator changes bundle membership, bundle role composition, or a mapping rule, the backend MUST enqueue outbox rows that project each affected user's resulting **effective-role closure delta** into Zitadel with `source='bundle'|'rule'` and `source_ref` pointing at the originating bundle or rule. A role in the delta that the change moved only through the mapping rules (its base holding did not move) is the rule's row wherever it is written, a bundle's cascade, publish or move, the planner or `FollowRules`: `source='rule'`, `source_ref` = the rule that reaches it (in the closure after for an add, before for a revoke), whose first role is the role that set it off (architecture-deepening task 1.6, decision 7). The delta is `adds = after−before` / `revokes = before−after` of the user's effective closure (direct grants ∪ bundle roles, folded through the active mapping-rule fixpoint — the same computation `collectUserRoles` uses read-side), computed from a pre-mutation read plus a pure simulation of the change, never a post-mutation read. This replaces today's read-side-only computation, under which bundle/rule changes projected nothing, and (post-ship fix, Task 25) replaces an earlier literal-bundle-roles-only cascade that missed rule-derived targets and grant-index-only holder discovery (findings P1a/P1b).

#### Scenario: Adding a user to a bundle cascades one outbox row per role in the resulting closure delta

- **WHEN** an operator calls `POST /api/v1/users/{id}/bundles` for a bundle containing N literal roles
- **THEN** the backend MUST enqueue one outbox row (`op_type='add'`) per role newly present in the user's effective closure — the bundle's N literal roles (`source='bundle'`, `source_ref=<bundle.id>`), PLUS any role an active mapping rule derives from one of those N roles that the user did not already effectively hold (`source='rule'`, `source_ref=<rule.id>`)
- **AND** a role the user already effectively held before the assignment (via another bundle, a direct grant, or a rule) MUST NOT be re-enqueued — the delta is empty for it

#### Scenario: A bundle role that is itself a mapping-rule source cascades its derived target too

- **WHEN** a bundle granting role A is assigned to a user, and an active mapping rule maps A to a target role B
- **AND** the user did not already effectively hold B
- **THEN** the backend MUST enqueue an add for BOTH A and B: A attributed to the bundle (`source='bundle'`, `source_ref=<bundle.id>`), B to the rule (`source='rule'`, `source_ref=<rule.id>`), so Pending changes says "Rule from A" and Change history lists B "by the automatic rule from A" under the bundle's entry
- **AND** removing the bundle before either was sent MUST cancel both queued adds and send nothing: a bundle's revoke answers the bundle's own queued adds, a rule's revoke answers every queued rule add of that person and role (`db.Answers`)
- **AND** this MUST hold regardless of whether the user already has any Zitadel-side grant recorded in the grant index — holder discovery reads Syndra's own bundle/direct-grant tables, not the grant index

#### Scenario: Closure coverage suppresses a revoke when another source still grants the role

- **WHEN** a bundle change would otherwise revoke a `(user, project, role)` grant
- **AND** the same triple remains in the user's effective closure after the change (still covered by another bundle, a mapping rule, or a direct grant)
- **THEN** the revoke MUST be suppressed — no outbox row is enqueued for that triple — and the role MUST persist
- **AND** the bundle/rule membership change (the "originating source was removed") still applies on its own table, independent of the suppressed grant

### Requirement: A direct write MUST carry its mapping-rule consequences through the outbox, in its own transaction

Every direct write (`db.enqueueWrites`: a direct grant, a replace or revoke by grant id, an approved request, a bulk give or extend, an adopted drift) MUST run through `db.FollowRules`. It reads the person's effective closure (direct grants ∪ pinned bundle versions − queued revocations, folded through `access.Closure`) before and after the write, inside the write's transaction and under the access-mutation lock. It MUST enqueue the difference, beyond the write's own base change, as outbox rows with `source='rule'`, `source_ref` = a rule reaching the role, and no ledger row. `internal/access` (`Closure`, `Ends`, `Fired`) is the only implementation that follows rules: the claims compiler, every cascade, the access views, drift and the mapping cohort call it (change `rules-on-the-pipeline`, ADR-006; one-path W2a). A rule-derived role ends when the last of its sources ends. Drift explains a rule target only when its source is in Syndra's records.

#### Scenario: Syndra's own grant reaches the end of a chain

- **WHEN** Syndra gives A directly, and active rules map A→B and B→C
- **THEN** the same transaction MUST enqueue `add` rows for B and C with `source='rule'`, pending before anything is sent
- **AND** the roles enqueued MUST equal what the claims compiler derives for that person

#### Scenario: Removal follows the chain and keeps what is held another way

- **WHEN** a direct grant of A is revoked, and C is also held through a bundle or a direct grant
- **THEN** a `revoke` row MUST be enqueued for B and none for C

#### Scenario: The webhook fires no rule

- **WHEN** a grant event survives the self-mutation guard (a change made outside Syndra)
- **THEN** no mapping rule MUST fire from it. It is drift, and adopting that drift is a direct write, which fires the rules.

### Requirement: Only a role that waits MUST hold a cascade's change

Since the switch over (migration 000058, `confirmation.model = 'roles'`), a row waits in Pending changes only when a role it carries is marked in Roles that wait. Every other row is delivered at once. Bundles and rules carry no confirmation mode: it was inert under roles and was dropped by migration 000078, which refuses to run unless the model is `roles`.

#### Scenario: A rule or bundle change delivers its rows unless the role waits

- **WHEN** a cascade enqueues rows and none of their roles waits
- **THEN** the backend MUST deliver them immediately, the rows having been persisted first so a crash mid-drain is recoverable
- **AND** a row whose role waits MUST stay `pending` with `wait_reason='role_waits'`

#### Scenario: Expiry sweep revocation bypasses the outbox and confirmation mode entirely

- **WHEN** the expiry scheduler revokes an expired grant's Zitadel role
- **THEN** the sweep MUST call the Management API directly (best-effort), NOT enqueue a `pending_zitadel_propagations` row
- **AND** the revocation therefore MUST NOT appear in "Recent automated cascades" (which lists only applied outbox rows with `source IN ('bundle','rule','lifecycle_cascade')`)
- **AND** `source='lifecycle_cascade'` remains reserved in the outbox `source` enum for a future outbox-routed lifecycle trigger; nothing writes it yet

