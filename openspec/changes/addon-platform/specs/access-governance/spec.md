## ADDED Requirements

### Requirement: Allowances MUST be an explicit third access band, never inferred

Access on an add-on target derives from a Zitadel role the operator maps to target resources. Anything beyond that — a quota, a specific path, a restriction — MUST be recorded as an explicit Syndra allowance with an actor and a timestamp, and MUST NOT be inferred from role membership. The access lineage surface MUST present allowances as a band distinct from source roles and derived grants, so the question "why does this user have access to X" resolves to exactly one of: the role grants it, a rule derived it, or a named operator granted it.

#### Scenario: Lineage attributes access to exactly one origin

- **WHEN** an operator inspects a subject's access on a target
- **THEN** each entitlement MUST be attributed to a source role, a derivation rule, or an explicit allowance
- **AND** an allowance MUST display the granting actor and the time it was granted

#### Scenario: Allowances are not synthesized from roles

- **WHEN** a subject holds a target-granting role and no allowance is recorded
- **THEN** the allowance band MUST be empty for that subject
- **AND** the backend MUST NOT create an allowance as a side effect of the role grant

### Requirement: Subtractive allowances MUST carry an expiry

An allowance that removes access the subject's role would otherwise grant MUST have an expiry and MUST be treated as a time-boxed suspension. A permanent reduction MUST be expressed by changing the role mapping, not by an open-ended denial, so a held role remains a truthful statement of access.

#### Scenario: Subtractive allowance without an expiry is rejected

- **WHEN** an operator submits a subtractive allowance with no expiry
- **THEN** the backend MUST reject it
- **AND** the error MUST direct the operator to change the role mapping for a permanent reduction

#### Scenario: Expiring suspension restores role-derived access

- **WHEN** a subtractive allowance reaches its expiry
- **THEN** the expiry sweep MUST remove it and re-converge the subject to their role-derived entitlements
- **AND** the restoration MUST be recorded in the audit trail

#### Scenario: Carve-out is visible wherever the role appears

- **WHEN** a subject holds a role whose access is partially suspended by a subtractive allowance
- **THEN** every surface presenting that role for that subject MUST show the carve-out
- **AND** MUST NOT present the role's full access as effective

### Requirement: Unconfirmed revocations MUST escalate as a security finding

Because target propagation fails open, a revocation may be recorded in Syndra without being confirmed on the target. Unconfirmed revocations MUST be surfaced on a dedicated operator view alongside drift triage, and MUST escalate to a distinct, more urgent presentation once they exceed a configured age.

#### Scenario: Queued revoke is surfaced apart from queued grants

- **WHEN** a revocation is recorded but not confirmed on the target
- **THEN** it MUST appear on the unconfirmed-revocation surface
- **AND** MUST be counted apart from succeeded operations
- **AND** MUST NOT be presented with the same urgency as an unconfirmed grant

#### Scenario: Age threshold escalates the presentation

- **WHEN** an unconfirmed revocation exceeds the configured age threshold
- **THEN** the surface MUST present it as a live security finding rather than a pending task
- **AND** MUST state how long the subject has retained access beyond the recorded revocation
