> **Status:** Wave 2 · Part 4 delta — Per-source confirmation mode & cascade projection (Theme 2 core) | [< Index](../../../../INDEX.md) | [Feature Coverage](../../../mkauth-core-architecture/specs/feature-coverage.md)

# Requirement: Automation Policies (delta)

## ADDED Requirements

### Requirement: Bundle and mapping-rule changes MUST project their effective grants into Zitadel through the outbox

When an operator changes bundle membership, bundle role composition, or a mapping rule, the backend MUST enqueue outbox rows that project each affected user's resulting **effective-role closure delta** into Zitadel with `source='bundle'|'rule'` and `source_ref` pointing at the originating bundle or rule. The delta is `adds = after−before` / `revokes = before−after` of the user's effective closure (direct grants ∪ bundle roles, folded through the active mapping-rule fixpoint — the same computation `collectUserRoles` uses read-side), computed from a pre-mutation read plus a pure simulation of the change, never a post-mutation read. This replaces today's read-side-only computation, under which bundle/rule changes projected nothing, and (post-ship fix, Task 25) replaces an earlier literal-bundle-roles-only cascade that missed rule-derived targets and grant-index-only holder discovery (findings P1a/P1b).

#### Scenario: Adding a user to a bundle cascades one outbox row per role in the resulting closure delta

- **WHEN** an operator calls `POST /api/v1/users/{id}/bundles` for a bundle containing N literal roles
- **THEN** the backend MUST enqueue one outbox row (`op_type='add'`, `source='bundle'`, `source_ref=<bundle.id>`) per role newly present in the user's effective closure — the bundle's N literal roles, PLUS any role an active mapping rule derives from one of those N roles that the user did not already effectively hold
- **AND** a role the user already effectively held before the assignment (via another bundle, a direct grant, or a rule) MUST NOT be re-enqueued — the delta is empty for it

#### Scenario: A bundle role that is itself a mapping-rule source cascades its derived target too

- **WHEN** a bundle granting role A is assigned to a user, and an active mapping rule maps A to a target role B
- **AND** the user did not already effectively hold B
- **THEN** the backend MUST enqueue an add for BOTH A and B, attributed to the bundle (`source='bundle'`, `source_ref=<bundle.id>`)
- **AND** this MUST hold regardless of whether the user already has any Zitadel-side grant recorded in the grant index — holder discovery reads MkAuth's own bundle/direct-grant tables, not the grant index

#### Scenario: Closure coverage suppresses a revoke when another source still grants the role

- **WHEN** a bundle change would otherwise revoke a `(user, project, role)` grant
- **AND** the same triple remains in the user's effective closure after the change (still covered by another bundle, a mapping rule, or a direct grant)
- **THEN** the revoke MUST be suppressed — no outbox row is enqueued for that triple — and the role MUST persist
- **AND** the bundle/rule membership change (the "originating source was removed") still applies on its own table, independent of the suppressed grant

### Requirement: Each mapping rule and bundle MUST carry a confirmation mode that governs whether its cascade drains automatically

`mapping_rules` and `bundles` MUST each have a `confirmation_mode` (`auto` | `manual`), defaulting from `config_settings.global.default_rule_confirmation_mode`. Auto-mode cascades drain immediately; manual-mode cascades and operator point mutations wait for explicit resume. Expiry sweeps are hardcoded auto — their authoring is the pre-authorization, and there is no `confirmation_mode` to gate against.

#### Scenario: Auto-mode rule fire drains without operator intervention

- **WHEN** a mapping rule with `confirmation_mode='auto'` fires and enqueues outbox rows
- **THEN** the backend MUST drain those rows immediately without waiting for an operator resume
- **AND** the outbox rows MUST still be persisted first, so a crash mid-drain is recoverable

#### Scenario: Manual-mode rule fire waits in the Pending tier

- **WHEN** a mapping rule with `confirmation_mode='manual'` fires
- **THEN** the enqueued outbox rows MUST remain `status='pending'` and surface in the Pending Propagation tier alongside operator point mutations
- **AND** the rows MUST NOT drain until the operator explicitly resumes

#### Scenario: Global default applies to new rules unless overridden

- **WHEN** an operator creates a new mapping rule via the UI without specifying a confirmation mode
- **THEN** the rule MUST inherit `config_settings.global.default_rule_confirmation_mode`
- **AND** a bulk "Set confirmation mode" action on the Policies UI MUST update the selected rules' `confirmation_mode` in one transaction

#### Scenario: Expiry sweep revocation bypasses the outbox and confirmation mode entirely

- **WHEN** the expiry scheduler revokes an expired grant's Zitadel role
- **THEN** the sweep MUST call the Management API directly (best-effort), NOT enqueue a `pending_zitadel_propagations` row
- **AND** the revocation therefore MUST NOT appear in "Recent automated cascades" (which lists only applied outbox rows with `source IN ('bundle','rule','lifecycle_cascade')`) and MUST NOT be gated by any rule's or bundle's `confirmation_mode`
- **AND** `source='lifecycle_cascade'` remains reserved in the outbox `source` enum for a future outbox-routed lifecycle trigger; nothing writes it yet
