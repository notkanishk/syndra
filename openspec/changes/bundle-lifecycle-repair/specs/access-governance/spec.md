> **Status:** bundle-lifecycle-repair delta — a bundle is born usable, and its version surfaces cite what was approved | [< Index](../../../../INDEX.md)

# Requirement: Bundle Lifecycle Repair (delta)

## MODIFIED Requirements

### Requirement: A new bundle MUST be created with at least one role, published as v1

Supersedes the `bundle-versioning` scenario "A bundle with no published version
cannot exist", which required an **empty** v1 at creation.

`POST /bundles` MUST refuse a request naming no roles, and MUST write the
working copy and v1 from the same set in one transaction.

The v1 itself is unchanged and still required: every assignment pins a version.
What changes is that it MUST NOT be empty. A bundle that grants nothing cannot
be assigned to any effect, and the roles an operator adds afterwards are
reported as unpublished changes against a version that never described
anything — so the product spends the whole of a bundle's setup describing it
inaccurately.

Bundles created before this requirement MAY have an empty v1 and remain valid.
Emptying a bundle after creation remains allowed.

#### Scenario: Creating a bundle with no roles

- **WHEN** `POST /bundles` arrives with a name and no roles
- **THEN** it MUST be refused with a validation error naming the `roles` field
- **AND** no bundle and no version MUST be written

#### Scenario: v1 is what the operator described

- **WHEN** a bundle is created naming three roles
- **THEN** v1 MUST contain exactly those three
- **AND** the working copy MUST contain exactly those three
- **AND** the bundle MUST report zero unpublished changes

The last assertion is the point. v1 and the working copy are diffed to compute
the unpublished set, so any disagreement between them at creation is an edit
nobody made.

#### Scenario: Assigning a newly created bundle grants something

- **GIVEN** a bundle created with `laser/trained`
- **WHEN** it is assigned to somebody
- **THEN** the assignment MUST pin v1
- **AND** MUST project `laser/trained`

## ADDED Requirements

### Requirement: Reading a bundle's roles MUST distinguish what it contains from what it grants

`GET /bundles/{id}/roles` MUST answer the working copy — what the next version
will contain. `?published=true` MUST answer the latest published version — what
the bundle grants today.

Every surface that previews or performs an assignment MUST ask for the published
set. An assignment pins the published version and nothing else, so a preview
built from the working copy promises roles the apply will not grant.

Both MUST return an empty array rather than `null`.

#### Scenario: A preview of an assignment excludes unpublished edits

- **GIVEN** Lab Tech v2 grants `laser/trained`
- **AND** its working copy has added `cnc/trained`, unpublished
- **WHEN** an operator opens the panel that assigns Lab Tech to somebody
- **THEN** the preview MUST list `laser/trained` only
- **AND** MUST state that the bundle has an unpublished change which is not part
  of this assignment, and which version the assignment gives

The final clause is a requirement rather than a nicety: the operator added
`cnc/trained` minutes earlier, and a correct preview that omits it without
explanation reads as a stale screen.

#### Scenario: The bundle editor still edits the working copy

- **WHEN** the bundle workspace lists a bundle's roles
- **THEN** it MUST show the working copy, including unpublished edits

### Requirement: Publishing a version and moving holders MUST cite a durable approval

Both `POST /bundles/{id}/publish` and `POST /bundles/{id}/holders/move` MUST
persist what their rehearsal showed and MUST return the `plan_id` their apply
cites, on the same terms as every other rehearsed mutation (design §8). An apply
whose rehearsal reached anybody and which cites nothing MUST be refused, and
MUST write nothing.

Before this, neither issued an approval. The shared dialog refuses to enable
Apply without one, so every publish that reached a holder could be previewed and
never applied.

An approval issued on one of these surfaces MUST NOT be spendable on the other.

#### Scenario: A publish that reaches holders can be applied

- **GIVEN** 14 people hold the bundle
- **WHEN** publishing is rehearsed with `migrate: true`
- **THEN** the plan MUST carry a `plan_id`
- **AND** citing it MUST publish the version

#### Scenario: An apply citing nothing is refused

- **WHEN** `?apply=true` arrives with no `plan_id` and the rehearsal reaches somebody
- **THEN** it MUST be refused
- **AND** no version MUST be published and no holder repinned

#### Scenario: A bundle nobody holds needs no citation

- **GIVEN** nobody holds the bundle
- **WHEN** publishing is applied
- **THEN** the version MUST be published without a citation

There is no subject to approve, so no approval is issued, and requiring one that
cannot exist would refuse a legitimate act.

### Requirement: An approval MUST bind the cohort a version surface derived, not just its subjects

A publish's cohort is whoever holds the bundle; it is not named in the request.
Subject-level verification checks the subjects **on** the approval, so a person
assigned the bundle between the review and the apply is absent from it rather
than stale, and would be moved under an approval that never mentioned them.

The approval MUST therefore be bound to the reviewed cohort — each holder and
the version they stand on — and to what the new version will contain. A change
to either MUST refuse the citation.

The version contents are included because a working-copy edit can change what
the next version *is* without changing any holder's delta: adding a role every
holder already holds from a mapping rule moves nobody.

#### Scenario: A holder joining invalidates the approval

- **GIVEN** a publish was rehearsed while one person held the bundle
- **AND** a second person is given the bundle before the apply
- **WHEN** the operator applies the approval
- **THEN** it MUST be refused as computed for a different request
- **AND** no version MUST be published

#### Scenario: A holder's reviewed delta moving invalidates the approval

- **GIVEN** a row read "v2 → v3, LOSES laser"
- **AND** a direct grant of `laser` reaches that person before the apply
- **WHEN** the operator applies the approval
- **THEN** it MUST be refused as stale, naming that subject

### Requirement: A rehearsed apply that moves nobody MUST be possible where the act is not the rows

Where the operation acts on a thing rather than on the people listed in the
plan, an apply with no acting row MUST be offered. Publishing a version is such
an operation: it cuts the version future assignments pin, whether or not any
current holder moves.

Two publishes move nobody and MUST both be applyable:

- a bundle nobody holds yet;
- a publish answering "leave the current holders on the version they are on",
  which `bundle-versioning` already requires to be a publish rather than a
  no-op.

A plan the client cannot read — no per-subject rows, or none listed alongside a
non-zero total — MUST stay refused. It cannot be reviewed, so it cannot be
approved.

#### Scenario: Publishing while leaving the holders behind

- **GIVEN** 14 people hold the bundle and the operator chooses to leave them
- **WHEN** the plan is rehearsed
- **THEN** every row MUST read as no change
- **AND** the apply control MUST be available, labelled as publishing the version
- **AND** applying it MUST cite the approval the rehearsal issued

### Requirement: A control the operator cannot use MUST say why, in visible copy

Every disabled control on a rehearse-then-apply surface MUST render its reason
beneath it. Not a `title`: hover does not exist on touch and does not survive a
screenshot sent to a colleague.

The disabled state MUST be derived from that reason, so a control cannot be
blocked for a reason the screen does not give.

This applies to the apply control on the review step and to the control that
computes the plan on the compose step.

#### Scenario: An unanswered question is named

- **GIVEN** the bundle has holders and the migrate question is unanswered
- **THEN** the control that computes the plan MUST be disabled
- **AND** MUST state that the question about the existing holders needs an answer

#### Scenario: A preview that came back without an approval

- **GIVEN** a plan carrying rows arrived with no `plan_id`
- **THEN** the apply control MUST be disabled
- **AND** MUST state that the preview carries no approval and the operator should
  try again
