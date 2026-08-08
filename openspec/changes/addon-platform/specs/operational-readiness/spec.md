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

### Requirement: Read-only operation MUST be available without redeployment

Each add-on MUST support a configured read-only mode in which it serves state and health but refuses every mutating operation. Entering read-only mode MUST NOT require rebuilding or redeploying the add-on, and its state MUST be visible to operators.

#### Scenario: Read-only mode refuses writes and stays observable

- **WHEN** an add-on is placed in read-only mode and a mutating operation is dispatched
- **THEN** the add-on MUST refuse the operation without mutating the target
- **AND** MUST continue to serve state and health requests
- **AND** the operator surface MUST show the add-on as read-only rather than as failing
