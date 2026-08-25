## ADDED Requirements

### Requirement: An add-on's shutdown drain MUST be allowed to complete

An add-on marks itself draining on receiving a termination signal and allows a bounded period for an in-flight mutation to settle rather than be abandoned half-applied. The deployment's container stop timeout MUST exceed that period, so that the add-on always reaches its own deadline before any externally imposed one.

The container timeout MUST be the outer bound and MUST NOT be the binding one. Where it binds, the add-on is killed mid-settle and the mutation is abandoned in precisely the state the drain exists to prevent — a change applied at the target with no terminal record of it in Syndra. That outcome is indistinguishable from a clean stop by observation: the process is gone either way, and a mutation truncated mid-settle leaves the same silence as one that never began. It therefore MUST NOT be left to an operator to notice.

The two values MUST be tied by a check rather than by a comment. They live in different files, in different languages, owned by different concerns — the add-on's own source and the deployment manifest — which is the arrangement in which two internally-consistent definitions of one thing drift apart unobserved. The add-on's period MUST be expressed as a named value its own tests can read, and a guard MUST fail when the deployment's timeout does not exceed it.

This requirement applies to every add-on, not only the first. Each inherits the same shutdown path, and an add-on added without the corresponding deployment setting would inherit the truncation silently.

#### Scenario: A mutation is in flight when the add-on is stopped

- **WHEN** the add-on receives a termination signal while a mutation is settling
- **THEN** it MUST be allowed to complete its drain before being killed
- **AND** the mutation's terminal status MUST be recorded

#### Scenario: The deployment timeout is lowered below the add-on's

- **WHEN** the container stop timeout is set to or below the add-on's shutdown period
- **THEN** a guard MUST fail
- **AND** the failure MUST name both values and the file each came from

#### Scenario: A further add-on is deployed

- **WHEN** an add-on beyond the first is added to the deployment
- **THEN** it MUST carry a stop timeout exceeding its own shutdown period

#### Scenario: Nothing is in flight

- **WHEN** the add-on is stopped with no mutation settling
- **THEN** it MUST shut down without waiting for the full period
