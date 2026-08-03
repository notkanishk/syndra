> **Status:** ui-capability-gap-closure delta — the Trace column stops guessing | [< Index](../../../../INDEX.md)

# Requirement: Cascade Lineage on Audit Rows (delta)

## ADDED Requirements

### Requirement: An audit row MUST carry the cascade it set off, stamped by the code that mints it

`audit_logs.cascade_id` (migration `000023`) names the cascade an event produced,
matching `CascadeGroup.CascadeID` exactly.

It MUST be written by `enqueueCascadeRows` and nowhere else. That function
already mints one id per batch — it is what groups the writes one triggering
event produced — and the audit row describing the same event has no other way to
learn it. Every atomic `*AndEnqueue` function previously wrote its own audit row
on the line immediately above its call to `enqueueCascadeRows`, which made this
a convention eleven functions had to keep rather than an invariant.

A cascade audit is therefore a parameter, not a separate statement, and it is a
LIST: `MoveHoldersAndEnqueue` writes one row per person moved, and all of them
name the same cascade.

`cascade_id` MUST be NULL unless the writes will appear in Change history.

That is one predicate, `cascadeGroupVisible`, reading one list,
`cascadeGroupSources` — the same list `GetCascadeGroups` filters on, passed to
the query as a parameter rather than spelled out in it. A cascade id is a handle
into that screen and nothing else, so the stamp and the screen's filter cannot
be allowed to disagree.

Two cases therefore get NULL:

- **The event produced no writes.** An id with no rows behind it would link to a
  page with nothing on it.
- **The writes are not cascade-sourced.** `DeleteDirectGrantAndEnqueue` goes
  through `enqueueCascadeRows` because its ledger delete, audit row and outbox
  rows must commit together — not because it is a cascade. Its writes carry
  `source='direct'`, whose surface is Pending changes. This held true even for a
  direct removal that revokes a second role a mapping rule derived from it: the
  whole delta is attributed to the grant, which is an operator's own write.

#### Scenario: A direct grant is removed
- **WHEN** a direct grant is deleted and its revokes are enqueued with `source='direct'`
- **THEN** the `direct_grant.removed` audit row is written with `cascade_id` NULL
- **AND** the audit log shows no trace link for it, because Change history would exclude the write

#### Scenario: The stamp and the filter drift apart
- **WHEN** `GetCascadeGroups` inlines its own source list instead of reading `cascadeGroupSources`
- **THEN** the source-coherence guard fails, because that is how a stamped audit row came to link
  to a page that filtered its own write out

The column MUST be nullable and MUST NOT reference `pending_zitadel_propagations`.
Outbox rows are drained and eventually cleared; an audit row has to outlive the
queue that carried out its consequence.

#### Scenario: A rule change reaches four people
- **WHEN** a mapping rule is edited and four users' closures change
- **THEN** the `mapping_rule.updated` audit row and all four outbox rows carry the same `cascade_id`
- **AND** they commit in one transaction

#### Scenario: Eight holders move to a new version
- **WHEN** eight people are repinned onto a bundle version
- **THEN** eight `bundle.holder_moved` audit rows are written, all naming one cascade

#### Scenario: The change reached nobody
- **WHEN** a bundle nobody holds is deleted
- **THEN** the `bundle.deleted` audit row is written with `cascade_id` NULL

#### Scenario: A cascade mutation grows its own audit insert
- **WHEN** any of `cascade.go`, `bundles.go`, `grants.go`, `bundle_versions.go` writes an
  `INSERT INTO audit_logs` outside `enqueueCascadeRows`
- **THEN** the source-coherence guard fails, because that row would carry no cascade id

### Requirement: The Trace column MUST state only what the row carries

One function, `traceFor`, decides what the column may claim, and both surfaces
that render it — the audit log and a person's Activity tab — go through one
component. Two vocabularies would let one row trace to two different things on
two screens.

- A row with a `cascade_id` renders `c_XXXX` and links to
  `/operations/cascades?cascade=<id>` — that cascade and no other.
- A row without one renders the bundle or rule id it does carry, prefixed `b_`
  or `R_` to match Change history, and **does not link**. It is the same
  identifier the column showed before, with the `c_` prefix that misdescribed
  it and the link that went elsewhere both removed.
- Anything else renders a dash. `bundle.role_added` records `project/role`,
  which is not an identifier and MUST NOT be shortened into something that
  looks like one.

Every action the backend writes MUST have a sentence in `AUDIT_ACTIONS`. The map
falls through to the raw key, which is the right failure mode and a silent one —
six actions accumulated behind it (`bundle.updated`, `bundle.deleted`,
`bundle.version_published`, `bundle.holder_moved`, `mapping_rule.deleted`,
`access_request.withdrawn`), each rendering as a machine key on a page whose
whole purpose is to be readable. The map is checked against the Go sources
rather than against a list somebody has to remember to update.

#### Scenario: A new audit action ships without copy
- **WHEN** the backend writes an action `AUDIT_ACTIONS` does not name
- **THEN** the coverage test fails, naming the action and the file and line that writes it

Pre-`000023` rows MUST NOT be backfilled by timestamp proximity. A cascade
writes its audit and outbox rows at one instant, so the match would be mostly
right — and a lineage link that is mostly right on a record of who may operate
a laser cutter is worse than one that is absent.

#### Scenario: A real cascade
- **WHEN** an entry carries a cascade id
- **THEN** the label is that id and the link narrows Change history to it

#### Scenario: A row from before the column existed
- **WHEN** an entry has no cascade id but names a rule
- **THEN** the label is the rule id, prefixed `R_`, and it does not link

#### Scenario: A resource that is not an id
- **WHEN** an entry's resource is `project/role`
- **THEN** the column renders a dash

### Requirement: Change history MUST answer for one named cascade

`GET /api/v1/propagations/cascade-groups?cascade=<id>` returns that cascade
alone. The narrowing MUST happen in the query: the audit tail is walkable back
to the first day, so a trace link from an event older than the fifty most recent
cascades would otherwise land on a page that appears to say nothing happened.

When the named cascade has no outbox rows left, the page MUST say the writes
were carried out and cleared. It MUST NOT reuse the empty state that says
nothing has cascaded yet — something did, or there would be no audit row
pointing here.

#### Scenario: Following a trace
- **WHEN** an operator opens a trace link
- **THEN** Change history shows that one cascade, with a way back to all of them

#### Scenario: The writes have been cleared
- **WHEN** the named cascade has no rows left in the outbox
- **THEN** the page states that they were carried out and cleared, and that the audit entry
  remains the record of what happened
