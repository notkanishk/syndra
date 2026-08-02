> **Status:** bundle-versioning delta — an edit and its consequence become two steps | [< Index](../../../../INDEX.md)

# Requirement: Bundle Versioning (delta)

## MODIFIED Requirements

### Requirement: Editing a bundle MUST NOT change anybody's access

Adding or removing a role on a bundle writes its working copy and MUST enqueue
nothing. The response MUST say so and MUST return the resulting unpublished
diff.

Before this, an edit projected to every holder on save, which made editing a
bundle fourteen people hold a decision nobody could take in passing — and made
"reshape this for next term without touching the current cohort" impossible to
express.

#### Scenario: Adding a role to a bundle with holders

- **GIVEN** 14 people hold the Lab Tech bundle
- **WHEN** an operator adds `Printing Lab / trained` to it
- **THEN** no outbox row is enqueued and no drain runs
- **AND** all 14 keep exactly the access they had

## ADDED Requirements

### Requirement: A holder's access MUST resolve through the version they are pinned to

Every `user_bundle_assignments` row MUST carry a `version_id`, and every
computation of a person's effective access MUST resolve their bundle roles
through it — never through the bundle's working copy.

This covers the closure inputs (`userBaseHoldings` and its variants) and the
lineage view. A single path reading the working copy would report access the
person does not have.

New assignments MUST pin the latest published version.

#### Scenario: Somebody left on an older version keeps it

- **GIVEN** Ada is pinned to Lab Tech v2, which grants `laser/trained`
- **AND** v3 removes that role
- **WHEN** v3 is published without migrating her
- **THEN** Ada still holds `laser/trained`
- **AND** her person page names the source as "Lab Tech v2"

#### Scenario: A bundle with no published version cannot exist

- **WHEN** a bundle is created
- **THEN** an empty v1 MUST be published in the same transaction

Every assignment pins a version, so a bundle with none could not be assigned at
all.

### Requirement: Publishing a version MUST be rehearsed, and MUST ask about existing holders

`POST /bundles/{id}/publish` MUST return a plan; `?apply=true` MUST perform the
writes. The request MUST carry an explicit `migrate` decision, and the UI MUST
NOT default it when the bundle has holders.

The plan MUST be computed per holder from THAT holder's pinned version. After
successive publishes with `migrate: false`, holders can be spread across several
versions, and a plan computed from a single "current" version would be wrong for
all but one group.

Publishing a bundle whose working copy matches its latest version MUST be
refused.

#### Scenario: A plan states who loses what

- **GIVEN** v4 removes a role that three of eleven holders have no other source for
- **WHEN** publishing is rehearsed with `migrate: true`
- **THEN** those three rows MUST name the role they lose
- **AND** the other eight MUST read as no change, stating what still covers them

#### Scenario: Holders on different versions move different distances

- **GIVEN** holders are spread across v2 and v3
- **WHEN** v4 is rehearsed with `migrate: true`
- **THEN** each row MUST state that person's own move, "v2 → v4" or "v3 → v4"

#### Scenario: Leaving holders is a publish, not a no-op

- **WHEN** a version is published with `migrate: false`
- **THEN** the version MUST be written and MUST be what new assignments pin
- **AND** no outbox row MUST be enqueued and no holder repinned

### Requirement: Which version each person holds MUST be visible, and filterable

The product MUST show, without a second request from the row:

- per bundle, how many holders sit on each version, and how many are behind the latest;
- per person, which version of each bundle they hold, and whether a newer one exists;
- on the bundle list, that a bundle has unpublished changes or holders left behind.

The People filter MUST narrow by bundle and then by version. A version filter
with no bundle MUST be ignored rather than applied.

#### Scenario: Finding the people a publish left behind

- **GIVEN** v3 was published without migrating, leaving 11 people on v2
- **WHEN** an operator filters People by the bundle and then by v2
- **THEN** exactly those 11 MUST be listed
- **AND** the description MUST read "on v2 of the Lab Tech bundle"

#### Scenario: A version filter alone narrows nothing

- **WHEN** `?version=2` arrives with no `bundle`
- **THEN** it MUST be dropped, and the unfiltered list rendered

### Requirement: Moving holders between versions MUST be rehearsed

`POST /bundles/{id}/holders/move` MUST follow the same plan-then-apply contract.
Moving somebody BACKWARDS onto an older version MUST be allowed and MUST be
rehearsed identically — it is a legitimate decision, and it revokes.

The target version MUST belong to the named bundle.

#### Scenario: A version from another bundle is refused

- **WHEN** a move names a `version_id` belonging to a different bundle
- **THEN** the write MUST be refused

Repinning across bundles would resolve a person's access through roles the
bundle never contained, and nothing downstream would notice.
