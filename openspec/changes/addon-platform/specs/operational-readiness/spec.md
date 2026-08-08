## ADDED Requirements

### Requirement: Add-on reachability and target version MUST be operator-visible

Each registered add-on MUST report reachability, the product and version of the target it serves, and the time of its last successful state read. The operator surface MUST present an unreachable add-on distinctly from a reachable one with queued work, because the remedies differ. Silence MUST NOT be presented as health.

#### Scenario: Unreachable add-on is distinguished from a backlog

- **WHEN** an add-on is unreachable and another has queued propagations
- **THEN** the surface MUST present the two states distinctly
- **AND** MUST state for the unreachable add-on when it was last successfully reached

#### Scenario: Stale state reads are labelled

- **WHEN** an add-on serves target state from its last good snapshot because the target is unreachable
- **THEN** the response MUST carry the age of that snapshot
- **AND** the operator surface MUST label the data as stale rather than current

### Requirement: Target health MUST be surfaced without granting operators target credentials

Operators MUST be able to read the health of a target — capacity, alerts, service state — through Syndra, served by the add-on. Syndra MUST NOT require operators to hold target credentials or reach the target's own console to answer routine health questions.

#### Scenario: Health is readable through Syndra

- **WHEN** an operator opens the target's health surface
- **THEN** capacity, active alerts, and relevant service state MUST be shown
- **AND** the data MUST be retrieved by the add-on using its own credentials, not the operator's

### Requirement: Add-ons MUST have an operator-settable lifecycle state

Each add-on MUST support three configured states: `active`; `draining`, in which new mutating operations are refused while already-issued operations are allowed to settle; and `read_only`, in which every mutating operation is refused immediately. All three MUST serve state and health requests, MUST be settable without rebuilding or redeploying the add-on, and MUST be visible to operators as a deliberate state rather than as a failure.

#### Scenario: Read-only refuses writes and stays observable

- **WHEN** an add-on is in `read_only` and a mutating operation is dispatched
- **THEN** the add-on MUST refuse it without mutating the target
- **AND** MUST continue to serve state and health requests
- **AND** the operator surface MUST show it as read-only rather than as failing

#### Scenario: Draining lets issued work settle

- **WHEN** an add-on is placed in `draining` while operations are in flight
- **THEN** it MUST refuse newly dispatched mutating operations
- **AND** MUST allow the in-flight operations to reach a terminal state
- **AND** the operator surface MUST show when draining has completed, so credential rotation or a target upgrade can proceed safely

#### Scenario: A lifecycle refusal is queued, not failed

- **WHEN** a mutating operation is refused because the add-on is in `draining` or `read_only`
- **THEN** the backend MUST account for it as queued, exactly as it does for an unreachable add-on
- **AND** MUST NOT record it as failed
- **AND** the row MUST resume when the add-on returns to `active`, so a maintenance window cannot silently convert pending revocations into terminal failures

#### Scenario: A non-active state is not reported as unhealthy

- **WHEN** an add-on is in `draining` or `read_only`
- **THEN** health MUST report the state explicitly
- **AND** MUST NOT present it as unreachable or failing
