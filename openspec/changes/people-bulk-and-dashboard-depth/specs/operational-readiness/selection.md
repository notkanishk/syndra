> **Status:** people-bulk-and-dashboard-depth delta — one selection model, one rehearsal | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta) — bulk interaction

## ADDED Requirements

### Requirement: Every list with a bulk action MUST use one selection model

People and the drift triage queue had grown separate implementations of the same
interaction with different behaviour — one had select-all and a docked bar, the
other had neither — and the next list to need selection would have invented a
third. There MUST be a single selection primitive, and every list with a bulk
action MUST use it: People, the drift triage queue, the request queue, and
Review › Expiring access.

The model MUST support, on every such list:

- **Select-all across the whole scope**, not the rendered page. Where the scope
  is wider than the page, the bar MUST state the count in words and offer to
  narrow to what is on screen.
- **Shift-click range extension** from the last-clicked row. The anchor MUST be
  dropped when the id list changes, because a range spanning "where that row
  used to be" selects rows nobody pointed at. Extending MUST apply the anchor
  row's state across the range rather than flipping each row, so extending the
  same range twice is idempotent.
- **Drag painting.** The row under the pointer when the gesture begins decides
  whether the path is being ticked or unticked; it MUST NOT flip each row it
  crosses, or dragging back over the path would undo it and the result would
  depend on the route taken. A movement threshold MUST be met before a press
  becomes a drag, the gesture MUST NOT scroll the list, and the click the
  browser fires after the drag MUST NOT toggle the starting row a second time.
- **Keyboard parity.** Space toggles, Shift+Arrow extends the same range
  shift-click does, `a` selects all and `Esc` clears. Without this, shift-click
  would be a mouse-only capability and an accessibility regression.

A selection MUST survive the list growing (a "Load more") and MUST drop any row
that has left the list entirely, because a bulk action aimed at a row no longer
on screen is aimed at nothing.

#### Scenario: Selecting more than one page
- **WHEN** select-all is used on a list of 60 rows paging at 50
- **THEN** all 60 are selected
- **AND** the bar states the count and offers to select only the 50 shown

#### Scenario: A drag that doubles back over itself
- **THEN** the rows remain in the state the gesture's first row set
- **AND** the result does not depend on the pointer's route

#### Scenario: A drag on a list whose bulk action is destructive
- **THEN** the list does not scroll during the gesture
- **AND** only rows that were visible can be selected

#### Scenario: The rows change while a selection is live
- **WHEN** the list grows
- **THEN** the selection is kept
- **WHEN** a selected row disappears
- **THEN** it is dropped from the selection

### Requirement: The selection bar MUST be docked and MUST describe the selection

The bar MUST NOT be inserted into the document flow above the list: appearing
there pushes every row down the moment the first checkbox is ticked, moving the
row the operator was about to tick next out from under the cursor. Structure
does not move in response to data.

It MUST state the selection in words before offering any verb, and SHOULD state
what the selection is composed of where the composition changes the decision —
the drift queue names how many selected rows are safety-gated, because that is
what its own ordering keys on and a batch of twelve wiki roles is not the same
decision as one containing three laser-cutter roles.

#### Scenario: The first row is selected
- **THEN** the rows already on screen do not move

### Requirement: Every bulk write MUST be rehearsed through one shared surface

Bulk grants, drift resolution and request decisions MUST each return the same
plan shape and MUST be presented through the same rehearsal dialog. An operator
must not have to learn what "will change" looks like separately on each screen,
and a preview drawn by different code from the write it previews is a preview of
nothing.

Applying a batch of request decisions MUST run the same code path a single
decision runs. That sequence — conditional transaction, race guard, cache
rebuild, inline drain — is the part that must not diverge: a second
implementation that drifted would leave requests approved but ungranted, which
re-surfaces later as `mkauth_only` drift.

Bulk revoke of drift remains deliberately absent. Adopting and marking-external
are reversible bookkeeping; revoking removes real access from real machines, and
reading twelve consequences at once is not something anyone actually does.

#### Scenario: Resolving a batch of drift
- **THEN** the plan names each row and states what would happen
- **AND** rows somebody else already resolved are reported as such, not written again
- **AND** nothing is written until the plan is confirmed

#### Scenario: Approving a batch of requests
- **THEN** the plan states that each approval mints a direct grant
- **AND** an approval without an attributable reviewer is refused for the whole batch

### Requirement: Applying MUST reach Zitadel, and the queue MUST hold only work MkAuth owes

The pending-changes queue means one thing: mutations MkAuth intends to make to
Zitadel and has not made yet. Two paths were putting rows into it that did not
belong there, and both were read by the operator as the system contradicting
itself.

**Adoption MUST resolve its own outbox row.** Adopting drift is the operator
saying "Zitadel is right, MkAuth was wrong" — there is no mutation owed
upstream. The `add` row exists only so adoption shares one code path with every
other ledger write, and it MUST be drained in the same request, exactly as
revoke already was. Left pending it is not merely noise: forty adopted roles
appear in the queue as forty writes MkAuth owes Zitadel, so accepting what
Zitadel already had reads as a queue of writes back to Zitadel. And a pending
`add` is a live instruction — an operator who adopts a role and then removes it
in Zitadel by hand gets it re-created by the next drain.

**Applying a bulk operation MUST project each row upstream.** `?apply=true` is
the operator authorising the write they just rehearsed; it MUST NOT mean "wrote
it down". A bulk removal reported as applied while the roles were still live in
Zitadel is the sharp end — the screen and the door disagree. The single-person
handlers already drained on `?apply=true`; the bulk path MUST match them. Bundle
operations are the one exception: their cascade drains according to the bundle's
own confirmation mode, and the bulk path MUST NOT override an owner who set that
bundle to manual.

A drain that fails MUST NOT undo the committed ledger write. The row stays
pending, which is then the honest state — MkAuth could not reach Zitadel to
confirm, so the change genuinely is still owed — and the next drain reclaims it.

#### Scenario: Adopting unexplained access
- **THEN** the drift is recorded as a direct grant
- **AND** its outbox row is resolved in the same request
- **AND** it does not appear in pending changes

#### Scenario: Adopting while Zitadel is unreachable
- **THEN** the adoption still succeeds
- **AND** the row stays pending, to be reclaimed by the next drain

#### Scenario: Applying a bulk role removal
- **THEN** each removal's outbox rows are drained before the response
- **AND** a row the rehearsal blocked drains nothing

#### Scenario: Applying a bulk bundle assignment
- **THEN** the cascade decides whether to drain, from the bundle's confirmation mode

### Requirement: Triage MUST offer selection by cluster

Drift arrives in clusters — one misconfigured rule, one person onboarded by
hand, one project nobody told MkAuth about. Each row MUST offer to select the
rows related to it, because no range gesture finds a cluster as reliably as
asking for one.

#### Scenario: Selecting similar
- **WHEN** "Select similar" is used on a row
- **THEN** the selection becomes every queued row sharing that row's person or project
