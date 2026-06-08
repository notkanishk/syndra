> **Status:** Wave 2 · Part 4 delta — Per-source confirmation mode & cascade projection (Theme 2 core) | [< Index](../../../../INDEX.md) | [Feature Coverage](../../../mkauth-core-architecture/specs/feature-coverage.md)

# Requirement: Automation Policies (delta)

## ADDED Requirements

### Requirement: Bundle and mapping-rule changes MUST project their effective grants into Zitadel through the outbox

When an operator changes bundle membership, bundle role composition, or a mapping rule, the backend MUST enqueue outbox rows that project the resulting `(user, project, role)` grants into Zitadel with `source='bundle'|'rule'` and `source_ref` pointing at the originating bundle or rule. This replaces today's read-side-only computation, under which bundle/rule changes projected nothing.

#### Scenario: Adding a user to a bundle cascades one outbox row per bundle role

- **WHEN** an operator calls `POST /api/v1/users/{id}/bundles` for a bundle containing N roles
- **THEN** the backend MUST enqueue up to N outbox rows (`op_type='add'`, `source='bundle'`, `source_ref=<bundle.id>`), one per role
- **AND** any role that already exists in Zitadel for that user (per the grant index) MUST self-resolve to `applied` without a redundant Management API call

#### Scenario: Removing coverage checks for another source before revoking

- **WHEN** a bundle change would revoke a `(user, project, role)` grant
- **AND** the same triple is still covered by another bundle, a mapping rule, or a direct grant
- **THEN** the revoke MUST be suppressed and the role MUST persist
- **AND** the intent ledger MUST record that the originating source was removed while the grant legitimately remains

### Requirement: Each mapping rule and bundle MUST carry a confirmation mode that governs whether its cascade drains automatically

`mapping_rules` and `bundles` MUST each have a `confirmation_mode` (`auto` | `manual`), defaulting from `config_settings.global.default_rule_confirmation_mode`. Auto-mode cascades drain immediately; manual-mode cascades and operator point mutations wait for explicit resume. Expiry sweeps and lifecycle-event cascades are hardcoded auto — their authoring is the pre-authorization — and surface in a "Recent automated cascades" element so they are never invisible.

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
