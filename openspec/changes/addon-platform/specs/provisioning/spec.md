## REMOVED Requirements

### Requirement: Orchestrated Sync Flow

**Reason**: The flow described a backend emitting LLDAP-shaped provisioning intents for a polling worker to claim. Both ends are removed: `provisioning_intents` carries `lldap_group`, and the worker that consumed it is deleted.

**Migration**: The propagation outbox carries target-dimensioned rows referencing desired-state snapshots, and the backend pushes to add-ons rather than being polled. The orchestration property the flow protected — webhooks reach only the backend, which decides, and the downstream component only executes — is preserved unchanged.

#### Scenario: Provisioning flows through the outbox, not an intent queue

- **WHEN** a validated event requires a target mutation
- **THEN** the backend MUST record an outbox row referencing a desired-state snapshot
- **AND** no `provisioning_intents` row MUST be emitted

### Requirement: Compensating Revocations on Partial Failure

**Reason**: Compensating revocation was needed because LLDAP membership was applied as individual add and remove operations, so a partial failure left a half-applied set that had to be undone.

**Migration**: Entitlement application is level-triggered — the target converges to a full desired set, so a failed apply leaves the previous state intact and the next apply converges. There is no half-applied state to compensate for. Ordering hazards are handled by per-`(subject, target)` serialization and stale-version rejection instead.

#### Scenario: A failed apply needs no compensation

- **WHEN** an entitlement apply fails partway
- **THEN** the target MUST retain its prior state or converge on retry
- **AND** no compensating revocation MUST be issued

### Requirement: Zitadel Grant Reconciliation

**Reason**: Superseded rather than deleted. Reconciliation is no longer a provisioning concern; it is the target-dimensioned drift sweep.

**Migration**: `services/drift` and its triage surface, now filtered by target. Zitadel behaviour is unchanged.

#### Scenario: Reconciliation lives with drift, not provisioning

- **WHEN** grant state is reconciled against a source of truth
- **THEN** the drift sweep MUST own it for every target
- **AND** no provisioning-specific reconciliation path MUST remain
