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

**Adoption MUST NOT create an outbox row at all.** Adopting drift is the
operator saying "Zitadel is right, MkAuth was wrong" — there is no mutation owed
upstream, and the outbox encodes one intent only, *make it so*. There is no
opcode for "confirm it is there", so an `add` row is not a receipt of the
adoption, it is a live instruction to perform it. Left pending it also reads as
debt MkAuth owes: forty adopted roles appear in the queue as forty writes back
to the system they were adopted from.

Draining that row in the same request is NOT sufficient and MUST NOT be treated
as the fix. It narrows the window rather than closing it — a drain that cannot
reach Zitadel leaves the row behind — and in that window an operator who adopts
a role and then removes it upstream by hand gets it re-created. Adoption writes
the ledger row and the audit row, which are the durable intent and the durable
trace, and nothing else.

The verification a drain would have bought is relocated, not lost: if the grant
vanished between detection and adoption, the ledger is what now disagrees with
Zitadel, the next reconcile raises it as `mkauth_only` drift, and a human
triages it. Surfacing that beats silently re-granting it.

**Applying a bulk operation MUST project each row upstream, and MUST report
whether it landed.** `?apply=true` is the operator authorising the write they
just rehearsed; it MUST NOT mean "wrote it down". A bulk removal reported as
applied while the roles were still live in Zitadel is the sharp end — the screen
and the door disagree.

Reporting is half the requirement. A row MUST be marked applied only when
Zitadel confirmed it, and a row that was recorded but not confirmed MUST carry a
distinct state saying so, with the reason. This state is neither success nor
failure: nothing was lost, the outbox will re-drive it, and the operator needs
to know the change has not taken effect yet. It MUST be counted apart from
success so a headline cannot round it up, and the confirmation the operator
reads MUST NOT announce success when it is non-zero.

A drain reports not-landing two ways — an error, or a halt with no error — and
both MUST be treated as not-yet-applied. Anything other than a confirmed apply
MUST be reported conservatively: under-claiming sends an operator to a queue
that turns out to be empty, over-claiming tells them a door is locked when it is
open.

Bundle operations MUST answer the same question through their own cascade, which
drains according to the bundle's confirmation mode. The bulk path MUST NOT
override an owner who set that bundle to manual — but a bundle that applies on
confirmation MUST be reported as not-yet-applied rather than as applied.

A drain that fails MUST NOT undo the committed ledger write.

#### Scenario: Adopting unexplained access
- **THEN** the drift is recorded as a direct grant
- **AND** no outbox row is written
- **AND** nothing appears in pending changes

#### Scenario: A role adopted, then removed upstream by hand
- **THEN** nothing re-creates it
- **AND** the divergence surfaces as mkauth_only drift for triage

#### Scenario: Applying a bulk role removal
- **THEN** each removal's outbox rows are drained before the response
- **AND** a row the rehearsal blocked drains nothing

#### Scenario: Applying while Zitadel is unreachable
- **THEN** the rows are reported as recorded but not yet in Zitadel, with the reason
- **AND** they are counted apart from the applied rows
- **AND** the confirmation does not announce success

#### Scenario: Applying a bulk bundle assignment
- **THEN** the cascade decides whether to drain, from the bundle's confirmation mode
- **AND** a bundle that applies on confirmation is reported as not yet applied

### Requirement: Triage MUST offer selection by cluster

Drift arrives in clusters — one misconfigured rule, one person onboarded by
hand, one project nobody told MkAuth about. Each row MUST offer to select the
rows related to it, because no range gesture finds a cluster as reliably as
asking for one.

#### Scenario: Selecting similar
- **WHEN** "Select similar" is used on a row
- **THEN** the selection becomes every queued row sharing that row's person or project
