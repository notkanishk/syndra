> **Status:** ui-capability-gap-closure delta — the expiring-access queue can record a decision | [< Index](../../../../INDEX.md)

# Requirement: Acknowledged Expiry (delta)

## ADDED Requirements

### Requirement: An operator MUST be able to record that a grant should be left to lapse

`POST /api/v1/review/expiring-grants/{grantId}/acknowledge` stores one
acknowledgement per grant: who, when, the expiry it was made against, and an
optional note. `DELETE` on the same path takes it back.

It MUST change nothing about the access. The expiry sweep still removes the grant
on its date. What it changes is the queue: "nobody has looked at this" and
"somebody looked and decided" stop being indistinguishable, which was the whole
of C4.

Both routes are operator-gated. **Any** operator may clear **any**
acknowledgement — the queue is shared and so is the decision. The row names who
made it, which is what makes it accountable; requiring the same person would
leave a decision unrevisable the moment they leave for the summer.

Acknowledging MUST be per-row and MUST NOT be offered in bulk. The record's value
is that a person read the row, and a gesture that acknowledges twelve at once
destroys exactly that. Bulk **extend** stays, because extending is a change to
access and reviewing twelve of those together is the work this screen exists for.

#### Scenario: A decision is recorded
- **WHEN** an operator acknowledges an expiring grant
- **THEN** the acknowledgement is stored with their id, the moment, and the grant's current expiry
- **AND** the response states that the grant still lapses on its date
- **AND** `grant_expiry.acknowledged` is audited against the person whose access it is

#### Scenario: A decision is taken back
- **WHEN** any operator clears an acknowledgement
- **THEN** the row returns to the undecided part of the queue
- **AND** `grant_expiry.acknowledgement_cleared` is audited

#### Scenario: There was nothing to take back
- **WHEN** clear is called on a grant with no acknowledgement
- **THEN** the response is `404`, saying so

### Requirement: An acknowledgement MUST stop applying when the grant changes

The reopen rule is **"when the grant changes"**, and it is implemented by storing
what was acknowledged rather than by invalidating anything.
`grant_expiry_acknowledgements.acknowledged_expires_at` records the date the
decision was about, and the read returns the acknowledgement only while it still
equals the grant's `expires_at`.

There MUST be no trigger, no sweep and no invalidation path. Validity is a
comparison in the read, which means it cannot go stale, cannot be forgotten by a
future write path, and can be verified without a database.

An operator agreed to let a specific role lapse on a specific day.
`direct_role_grants` is UNIQUE on `(user, project, role)` and upserts in place, so
extending or re-granting keeps the grant's id and moves its date — the case this
catches. They have not signed off on the new date, so the row asks again.

The write side MUST check the submitted date against the row, under
`FOR UPDATE`, and refuse a mismatch with `409`. Storing an acknowledgement against
a date the grant no longer carries would be accepted, never apply, and leave the
operator believing they had recorded something. `expires_at` is therefore
REQUIRED on the request: without it there is nothing to measure the rule against,
and the acknowledgement would be permanent.

The table MUST hold at most one row per grant, and MUST cascade on the grant's
deletion — unlike `audit_logs.cascade_id` (000023) and
`onboarding_triggers.bundle_id` (000021), which are history and must outlive their
subject. This is an annotation on a live row, meaningless once the grant is gone,
and the grant being swept away on its date is the normal end of its life. The
history of who decided what lives in `audit_logs`.

#### Scenario: The grant is extended after being acknowledged
- **WHEN** an acknowledged grant's `expires_at` is moved
- **THEN** the acknowledgement stops being returned and the row is undecided again
- **AND** nothing had to notice the change

#### Scenario: The page was loaded before somebody extended it
- **WHEN** an operator acknowledges a date the grant no longer carries
- **THEN** the response is `409` telling them to reload before deciding
- **AND** nothing is stored
- **AND** the console's dialog stays open, because a closed one reads as a saved decision

#### Scenario: The grant lapses
- **WHEN** the expiry sweep deletes the grant
- **THEN** its acknowledgement goes with it, and the audit row remains

#### Scenario: The comparison is removed
- **WHEN** the acknowledgement join stops comparing `acknowledged_expires_at` to `expires_at`
- **THEN** the source guard fails, because every acknowledgement would silently become permanent

### Requirement: The queue MUST separate decided rows from undecided ones without hiding them

Acknowledged rows move below a counted heading. They MUST NOT be removed from the
screen: hiding them client-side is the failure the design brief named — a shared
queue that diverges per operator is worse than a noisy one — and it would also
hide a decision from the person who made it.

Each acknowledged row MUST name who decided and when, and show the note if there
is one. The next operator must be able to see whose judgement they would be
overriding.

**Extend MUST remain available on an acknowledged row.** Changing one's mind
toward keeping somebody's access must never be harder than the decision that lets
it go. An acknowledged row carries no selection checkbox, so a bulk extend cannot
undo a decision by accident.

A queue with nothing undecided MUST say so outright — it is otherwise identical on
screen to a queue nobody has touched.

The console MUST state, where the button is, that acknowledging does not keep the
access and that the acknowledgement reopens if the grant changes. Both facts are
counter-intuitive and neither is discoverable by trying it.

#### Scenario: Nothing is waiting
- **WHEN** every grant in the window is acknowledged
- **THEN** the screen says nothing is waiting on a decision, and still lists the acknowledged rows

### Requirement: A bulk extend MUST act on what was selected, and the queue MUST NOT survive it

`POST /api/v1/grants/bulk` accepts `grant_ids` on `extend`, narrowing the write to
those grants. Omitted, it extends every expiring direct grant the named people
hold.

Both meanings are needed, and the difference is the screen. **People** is a list
of people, and "extend their expiring access" is exactly what selecting them
asks for. **Review › Expiring access** is a list of *grants*, and reducing its
ticked rows to user ids extends grants the operator never saw — other projects,
and dates beyond the 30-day window the screen is scoped to. Any screen whose rows
are grants MUST pass `grant_ids`.

`grant_ids` is rejected on every other op. Accepted and ignored, it would let a
caller believe they had scoped an operation they had not.

The id set is flat across the selected people, and the rehearsal applies it
per-person. A grant id belongs to exactly one person, so one person's selection
cannot reach another's access; an id that is not theirs matches nothing. A person
with nothing selected MUST be reported as such, distinctly from a person with
nothing expiring — an operator reading a plan needs to know which.

Every grant write that can change an expiry MUST invalidate the expiring-access
queue. `POST /users/{id}/grants` upserts on `(user, project, role)`, so it is both
"grant" and "extend"; the bulk apply rewrites the same dates. A stale queue after
an extension shows the row with its old date, and — if it had been acknowledged —
an acknowledgement the new date has already voided. The console would be
contradicting the backend about who keeps access.

#### Scenario: One row ticked, one grant extended
- **WHEN** an operator ticks one row for somebody who holds two expiring grants
- **THEN** only the ticked grant is extended
- **AND** the grant outside the review window is untouched

#### Scenario: A selection cannot cross between people
- **WHEN** two people are in the plan and only one person's grant is selected
- **THEN** the other person's row reports no change, saying it was not selected

#### Scenario: Selecting people still means all their expiring access
- **WHEN** `extend` is submitted with no `grant_ids`
- **THEN** every expiring direct grant those people hold is extended

#### Scenario: The queue reflects an extension immediately
- **WHEN** a grant is extended, individually or in bulk
- **THEN** the expiring-access queue and the governance counts are refetched
- **AND** an acknowledged row that was extended no longer shows its acknowledgement

#### Scenario: The dialog explains itself
- **WHEN** an operator opens the acknowledgement dialog
- **THEN** it states that the grant still lapses, that the queue stops asking, and that the
  acknowledgement ends if the grant is extended or re-granted
