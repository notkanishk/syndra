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

### Requirement: A successful mutation MUST be read back before it can be recorded as a base

An add-on MUST read the account as the target holds it after every successful mutation, and MUST derive both the returned fingerprint and the returned observed values from that read. A value the add-on assembled from what it asked for MUST NOT be reported as observed state, on any path: it agrees with the request by construction, and recording it as a base reproduces exactly the failure a base exists to prevent.

The observed managed-field values MUST travel in the apply response and MUST be declared in the contract artifact, so that neither side sends a field the other's decoder does not know.

A read-back that fails MUST NOT produce a base. The mutation happened and MUST be reported as it is today; the response MUST carry no observed values, and the backend MUST record none. A subject in that state MUST be treated exactly as one that has never been applied.

#### Scenario: An account that already exists is updated

- **WHEN** an apply updates an account the target already holds
- **THEN** the fingerprint and the observed values MUST come from a read performed after the write
- **AND** MUST NOT be assembled from the requested values

#### Scenario: The write succeeds and the read-back fails

- **WHEN** a mutation succeeds and the account cannot then be read
- **THEN** the response MUST carry no observed values
- **AND** no base MUST be recorded
- **AND** the subject MUST converge on the next pass as one with no base

### Requirement: A conflict MUST NOT be resolved without a person

An unattended pass MUST apply only differences whose cause it can determine, and MUST NOT resolve a conflict under any configuration. A setting that resolved conflicts automatically would restore the behaviour this change removes, and it would be enabled on the deployment where being wrong is most expensive.

Resolution MUST offer keeping the desired state. It MUST offer adopting the target's value ONLY where that value can be held for the one subject by a desired-state source that already exists; where it cannot, the surface MUST instead name the policy that produces the value and how many people that policy reaches, and the resolution MUST be an edit to that policy.

Adopting MUST change the desired state rather than suppressing the finding: a suppressed finding returns on the next pass, which teaches an operator that the surface is noise. A finding whose only honest resolution is a policy change MUST NOT be dismissible, for the same reason.

A per-subject additive entitlement MUST NOT be introduced to make a resolution expressible. An entitlement granted outside the role model is one no access review reaches, and a conflict dialog is not a reason to create that class.

#### Scenario: The target's value can be held for one subject

- **WHEN** an operator adopts a target value that a per-subject subtractive decision can express
- **THEN** the desired state MUST change through that mechanism, carrying its author, its reason and its bound in time
- **AND** the next pass MUST report agreement rather than the same finding

#### Scenario: The target's value has no per-subject owner

- **WHEN** the differing field is derived from a policy that applies to every holder of a role
- **THEN** the surface MUST NOT offer to adopt it for the one subject
- **AND** MUST name the policy and the number of people it reaches
- **AND** the finding MUST remain open until a pass agrees

### Requirement: Every unattended outcome that is not applied MUST become a durable finding

A pass that determines a difference it may not resolve MUST record it durably, not return it. `theirs-only` — the target changed and the desired state did not — MUST be a finding in its own right: it is what a hand edit on the target looks like, it is the most common of these states, and left as the return value of a pass it is visible only to whoever ran that pass.

Findings MUST be deduplicated per subject, target and field, so a pass on a schedule reports one finding for one unresolved difference rather than one per pass.

#### Scenario: A hand edit on the target is left unresolved across passes

- **WHEN** the same target-only difference is seen by several scheduled passes
- **THEN** exactly one finding MUST exist for it
- **AND** it MUST remain visible to an operator who did not run any of those passes
