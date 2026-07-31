> **Status:** basic-advanced-ia delta — direct-grant removal, person detail, member surface | [< Index](../../../../INDEX.md)

# Requirement: User Management (delta)

## ADDED Requirements

### Requirement: A direct grant MUST be removable through MkAuth's own ledger

`DELETE /api/v1/users/{id}/grants/{grantId}` MUST delete the `direct_role_grants` row, write the audit row, and enqueue the user's effective-access delta, all in a single transaction. The compiled cache MUST be rebuilt on a context detached from the request before the response is returned.

It MUST NOT be implemented as the Zitadel-side grant delete (`DELETE /api/v1/zitadel/users/{id}/grants/{grantId}`), which removes a different object and leaves the MkAuth row behind for the next cache compile to restore.

A grant that does not exist for that user MUST return 404, not 500: clicking remove twice, or clicking after the expiry sweep, is an ordinary race and not a server fault.

#### Scenario: The removal ends the access

- **WHEN** an operator removes a direct grant that was the role's only source
- **THEN** the ledger row MUST be gone
- **AND** a revoke MUST be queued in the outbox
- **AND** the user's compiled cache MUST have been rebuilt before the response

#### Scenario: Removing an already-removed grant

- **WHEN** the grant id names no row for that user
- **THEN** the response MUST be 404 with a message saying it may already have been removed or expired

### Requirement: Removing a direct grant MUST NOT revoke access the person still holds

The removal MUST enqueue the user's effective-role closure delta — the closure before the deletion versus the closure after it — computed from pre-mutation reads plus an in-memory simulation of the removal, exactly as the bundle-removal cascade does. It MUST NOT enqueue an unconditional revoke.

A role still carried by a bundle or produced by a mapping rule stays in the "after" closure and MUST NOT be revoked; a rule-derived role the removed grant alone supported falls out of the closure and MUST be revoked with it.

This is what makes the confirmation dialog's promise true. An unconditional revoke removed access from the identity provider that the person demonstrably still held, contradicting the sentence the operator was shown and taking the role away until the next compile restored it.

The response MUST report which roles were revoked and which were retained, so the caller can verify the promise the dialog made.

#### Scenario: A bundle still carries the role

- **GIVEN** a person holds `pLaser/trained` through a direct grant AND through the Lab Tech bundle
- **WHEN** the direct grant is removed
- **THEN** no outbox row MUST be queued at all
- **AND** the response MUST report `pLaser/trained` as retained

#### Scenario: A rule still produces the role

- **GIVEN** a person holds `p3D/operator`, a rule maps it to `pLaser/trained`, and they also hold `pLaser/trained` directly
- **WHEN** the direct grant of `pLaser/trained` is removed
- **THEN** no revoke MUST be queued for `pLaser/trained`

#### Scenario: The last source is removed

- **GIVEN** no bundle and no rule gives the person the role
- **WHEN** the direct grant is removed
- **THEN** exactly one revoke MUST be queued for that role
- **AND** the response MUST report it as revoked

#### Scenario: A role the grant alone supported goes with it

- **GIVEN** the removed grant is `pStudio/door` and a rule maps it to `pWiki/wiki-read`, which nothing else supplies
- **WHEN** the grant is removed
- **THEN** revokes MUST be queued for both `pStudio/door` and `pWiki/wiki-read`

### Requirement: The person detail page MUST group by project and read Granted before Automatic

Roles MUST be grouped by project, and within each project the granted roles MUST appear above the automatic ones — the things a human decided read first. Every row MUST carry its access source in the fixed order Direct → Via bundle → Automatic.

Where a role is held more than once, the page MUST state so plainly above the groups, because otherwise removing one source looks like it will remove the role.

`cleanup_hints` MUST render as advisory notes, never as errors.

#### Scenario: A doubly-held role is called out

- **GIVEN** a person holds a role through both a bundle and a mapping rule
- **WHEN** their page renders
- **THEN** a notice MUST state that the role is held twice and name both sources

### Requirement: Bundle membership chips MUST NOT be individually removable

Bundle chips on the person detail page communicate membership and nothing else. There MUST be no inline removal control. Removal MUST live behind an explicit Manage bundles surface that shows the impact first.

Inline removal invites a misclick that silently strips a dozen roles, and it stops scaling the moment somebody holds four bundles.

#### Scenario: No inline dismissal

- **WHEN** the bundles strip renders
- **THEN** no chip MUST carry a remove affordance

### Requirement: The member surface MUST express access as sentences, not vocabulary

The member's own access view MUST render the access source as plain language — "Because you're in Lab Tech", "Given to you until 2 Aug", "Comes with door access, automatically" — and MUST NOT show role keys, "derived", `effective_role_keys`, or the Direct / Via bundle / Automatic chips.

Operator-only affordances MUST NOT be rendered for members at all.

#### Scenario: A member sees no jargon

- **WHEN** a member views their own access
- **THEN** no role key MUST appear
- **AND** no access-source chip MUST appear
- **AND** no grant or bundle-management control MUST appear
