## ADDED Requirements

### Requirement: Member add-on surfaces MUST occupy a fixed destination with gated content

Member-facing add-on surfaces MUST live at their own member navigation destination, and that destination MUST be present for every member regardless of entitlement, because whether a member holds infrastructure access is data and navigation structure MUST NOT move in response to data.

The content MUST be gated on both entitlement and account existence. A member with no role mapped to any target MUST see an explanation rather than any credential or connection affordance. A member holding such a role whose target account does not yet exist MUST see that pending state, and MUST still not be offered a credential affordance, because a credential set cannot be queued and would be offering an action that can only fail.

#### Scenario: The destination exists regardless of entitlement

- **WHEN** a member with no infrastructure access views their navigation
- **THEN** the destination MUST be present
- **AND** it MUST NOT appear or disappear as that member gains or loses a mapped role

#### Scenario: No entitlement means no credential affordance

- **WHEN** a member holds no role mapped to any target
- **THEN** the surface MUST explain that they have no infrastructure access
- **AND** MUST NOT render a credential form or connection instructions

#### Scenario: Entitlement without an account shows the pending state

- **WHEN** a member holds a mapped role but their target account has not yet been created
- **THEN** the surface MUST show that provisioning is pending
- **AND** MUST NOT render a credential form

#### Scenario: The affordance appears once the account exists

- **WHEN** the member's target account exists
- **THEN** the credential affordance and connection instructions MUST render
- **AND** the fail-closed path MUST remain only as a backstop for an account removed between render and submission

### Requirement: Members MUST be able to set their own infrastructure credential

A member MUST be able to set and reset the credential their lab equipment requires, from their own Syndra surface, without operator involvement. The credential MUST be forwarded to the target and never retained by Syndra. Syndra MUST show whether a credential exists and when it last changed, and MUST NOT present the infrastructure credential as the member's primary account password.

#### Scenario: Member sets a credential and it is not retained

- **WHEN** a member submits a new infrastructure credential
- **THEN** the backend MUST forward it to the target add-on synchronously
- **AND** MUST record that the credential was set, by whom, and when, without the value
- **AND** MUST NOT store the value in any table, cache, or log

#### Scenario: Credential status is visible without exposing the secret

- **WHEN** a member views their infrastructure access
- **THEN** the surface MUST show whether a credential exists and its last change time
- **AND** MUST NOT display or transmit the credential value

#### Scenario: Credential is distinguished from the primary identity

- **WHEN** the infrastructure credential is presented to a member
- **THEN** the surface MUST state that it is scoped to lab infrastructure access only
- **AND** MUST NOT imply it governs Syndra or Zitadel sign-in

### Requirement: Members MUST see connection instructions with the account name the target uses

A member with access to a target MUST see the instructions needed to connect, including the account name the target actually uses for them, which may differ from their Syndra or Zitadel identity. Instructions MUST reflect the paths the member's current entitlements grant, and MUST NOT list resources they cannot reach.

#### Scenario: Instructions show the real target account name

- **WHEN** a member views connection instructions for a target
- **THEN** the surface MUST show the account name that target uses for them
- **AND** that name MUST come from the add-on's report of the account it created, not be re-derived by the frontend

#### Scenario: Instructions reflect current entitlements

- **WHEN** a member's entitlements on a target change
- **THEN** the listed resources MUST change to match
- **AND** MUST NOT list a resource the member's current entitlements do not reach

### Requirement: Target account lifecycle MUST follow role grants

A target account MUST be created when a subject first receives a role granting access to that target, and MUST NOT be created for subjects with no such role. Losing the last such role MUST disable access reversibly rather than deleting the account.

#### Scenario: First qualifying role creates the account

- **WHEN** a subject receives their first role granting access to a target
- **THEN** the backend MUST enqueue account creation on that target
- **AND** the created account name MUST be recorded against the subject

#### Scenario: Subjects without a qualifying role have no account

- **WHEN** a subject holds no role granting access to a target
- **THEN** no account MUST be created for them on that target

#### Scenario: A role qualifies only by virtue of a mapping

- **WHEN** the backend evaluates whether a grant change qualifies for a target
- **THEN** it MUST decide from the role-to-target mappings for that target
- **AND** a role with no mapping to a target MUST NOT trigger account creation on it

#### Scenario: Losing the last mapped role disables rather than deletes

- **WHEN** a subject loses their last role mapped to a target
- **THEN** the resolved entitlement set MUST mark the account and its service access disabled
- **AND** MUST NOT enqueue a deletion
- **AND** the account and its data MUST survive

#### Scenario: Regaining a mapped role re-enables through the same path

- **WHEN** the subject later regains a role mapped to that target
- **THEN** the resolved entitlement set MUST mark the account and its service access enabled
- **AND** the account MUST be restored without operator intervention and without a second account being created

### Requirement: Dormant target accounts MUST have a housekeeping surface

Because deprovisioning preserves accounts, accounts accumulate that are locked, roleless, or unused. The backend MUST provide an operator surface listing these accounts with the reason each is dormant and how long it has been so, supporting both individual and bulk action, with a plan shown before any apply.

#### Scenario: Dormant accounts are listed with their reason and age

- **WHEN** an operator opens the housekeeping surface for a target
- **THEN** each listed account MUST state why it is dormant and for how long
- **AND** accounts still held by an active role MUST NOT appear

#### Scenario: Bulk action is planned before it applies

- **WHEN** an operator selects multiple dormant accounts for an action
- **THEN** the surface MUST present the per-account effect before the apply is possible
- **AND** the apply MUST act on the planned rows rather than recomputing the selection
