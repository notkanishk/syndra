> **Status:** ia-screen-completion delta — role group as its own field, creation restored, bundle role removal | [< Index](../../../../INDEX.md)

# Requirement: Role Management (delta)

## ADDED Requirements

### Requirement: A role's group MUST be its own field

`ProjectRole.Group` MUST carry the identity provider's grouping. It MUST NOT be written into `Description`.

The two answer different questions — "Safety-gated" versus "can cut and engrave unsupervised" — and folding one into the other made every upstream role render its group where its description belonged, which is exactly the line an operator reads to make a decision.

The role catalogue MUST expose `group` and, where Syndra created the role, its clone provenance.

#### Scenario: An upstream role shows its group and no invented description

- **GIVEN** a Zitadel role with group `Safety-gated` and no description
- **WHEN** the roles index renders it
- **THEN** the Group column MUST read `Safety-gated`
- **AND** the description MUST be empty rather than repeating the group

### Requirement: Creating a role MUST be reachable from the roles index

`POST /api/v1/roles` MUST have an affordance. Creating a role through Syndra writes it locally and upstream in one action and rolls the local row back if the provider refuses; creating one directly in the provider is invisible to Syndra until the drift sweep flags it.

Clone-from MUST be offered, and MUST record provenance so a later reader can tell two similar roles are deliberately related rather than an accidental duplicate.

#### Scenario: A duplicate key is refused before submission

- **GIVEN** a role `trained` already in the chosen project
- **WHEN** the operator types `trained` as the key
- **THEN** the form MUST say so and MUST NOT allow submission

### Requirement: Removing a role from a bundle MUST show its impact before the click

`DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}` MUST have an affordance, and selecting it MUST replace the impact panel with a breakdown naming: how many holders lose the role through this bundle, who keeps it because a rule reproduces it, and what else is lost by cascade.

The breakdown occupies the space a confirmation dialog would have taken. The consequence belongs on screen before the commit, not after the click.

#### Scenario: A rule that reproduces the role is named

- **GIVEN** a bundle role that an automatic rule also grants
- **WHEN** the operator selects Remove
- **THEN** the impact panel MUST name the rule's input role and say those holders keep it

### Requirement: A rule MUST be editable, and its validation MUST name the chain

An existing rule MUST be editable in place, with per-rule confirmation mode. Save MUST be blocked until validation passes, and the screen MUST say so rather than leaving a dead control.

Validation MUST report how many people the rule would newly affect, and MUST name any rule it chains into. A rule that triggers another rule is the single most surprising thing this system does, and finding out afterwards is how people stop trusting it.

#### Scenario: Editing a field invalidates the previous verdict

- **GIVEN** a rule that has passed validation
- **WHEN** the operator changes its target role
- **THEN** the verdict MUST be cleared and save MUST be blocked again

#### Scenario: A chain is named before save

- **GIVEN** an existing rule whose input is the draft rule's output
- **WHEN** the draft is validated
- **THEN** the validation MUST name that rule and what it would additionally grant
