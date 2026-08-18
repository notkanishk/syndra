## ADDED Requirements

### Requirement: Reconciliation MUST distinguish who changed a value

Reconciliation MUST compare the target's state against BOTH the desired state and the state the target reported at the last successful apply. A comparison of desired against current cannot determine the cause of a difference, and a system that cannot determine the cause MUST NOT act as though it had: the same difference is produced by Syndra having moved, by the target having been changed by hand, by both having moved to the same value, and by both having moved to different values, and only the first is safe to resolve by writing.

The recorded base MUST be what the target reported after the apply, never what Syndra intended to write. A base derived from intent equals the desired state by construction and can never produce a conflict, which is the behaviour this requirement exists to replace.

Classification MUST be per managed field. A whole-account comparison reports a conflict between two changes that touched nothing of each other's, and fields the deployment does not manage for that subject MUST NOT participate at all.

A subject with no recorded base MUST converge exactly as it does without this mechanism. Inventing a base either fabricates agreement or raises a conflict for every managed subject at once, and neither is a statement about anything that happened.

#### Scenario: The target was changed by hand and Syndra was not

- **WHEN** the target's value differs from the base and the desired state matches it
- **THEN** the difference MUST be reported as drift for triage
- **AND** MUST NOT be overwritten by an unattended pass

#### Scenario: Both sides changed to the same value

- **WHEN** the target and the desired state agree but both differ from the base
- **THEN** no write MUST be issued
- **AND** the base MUST be updated, so the agreement is not re-detected on every later pass

#### Scenario: Both sides changed to different values

- **WHEN** the target and the desired state both differ from the base and from each other
- **THEN** the pass MUST record a conflict and resolve nothing
- **AND** the finding MUST carry all three values, since "what was it before" is the question an operator asks first

#### Scenario: The account is gone from the target

- **WHEN** a binding names an account the target no longer has
- **THEN** the pass MUST NOT recreate it
- **AND** MUST report it as a state requiring a decision between re-provisioning and unbinding

### Requirement: A conflict MUST NOT be resolved without a person

An unattended pass MUST apply only differences whose cause it can determine, and MUST NOT resolve a conflict under any configuration. A setting that resolved conflicts automatically would restore the behaviour this change removes, and it would be enabled on the deployment where being wrong is most expensive.

Resolution MUST offer keeping the desired state, adopting the target's value into the desired state, and setting a third value. Adopting MUST change the desired state rather than suppressing the finding: a suppressed finding returns on the next pass, which teaches an operator that the surface is noise.

#### Scenario: An operator adopts the target's value

- **WHEN** an operator resolves a conflict in the target's favour
- **THEN** the desired state MUST change to that value
- **AND** the next pass MUST report agreement rather than the same conflict
