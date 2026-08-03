> **Status:** ui-capability-gap-closure delta — nothing MkAuth creates is permanent | [< Index](../../../../INDEX.md)

# Requirement: Lifecycle — Retiring What Was Created (delta)

## ADDED Requirements

### Requirement: A mapping rule MUST be removable, and removing it MUST take back what only it granted

`DELETE /api/v1/rules/mapping/{id}` deletes the rule and enqueues the resulting
per-user revokes in ONE transaction.

The revoke set is the closure diff with the rule's edge removed — the same
computation `CascadeRuleUpdated` performs, with no replacement edge. This is not
an implementation convenience, it is the definition: a person keeps the target
role if any other source still produces it, and that question is answered per
person by folding the remaining rules over their holdings, never by inspecting
the rule alone.

The deletion inherits the rule's own `confirmation_mode`. A rule whose writes
queued for confirmation queues the writes that undo it.

#### Scenario: The rule was the only source
- **WHEN** a rule granting `Laser Lab / trained` is deleted
- **AND** a holder of its trigger role has no bundle or direct grant for that role
- **THEN** a revoke for `Laser Lab / trained` is enqueued for that holder
- **AND** the rule row and the revoke commit together

#### Scenario: Another source covers the target
- **WHEN** the same rule is deleted
- **AND** a holder also receives `Laser Lab / trained` from a bundle
- **THEN** nothing is enqueued for that holder

#### Scenario: The rule was removed concurrently
- **WHEN** the `DELETE` finds no row to delete
- **THEN** the transaction rolls back and the caller receives `404`
- **AND** no revokes are enqueued, because whoever won the race already enqueued them

### Requirement: A bundle MUST be renamable and retirable

`PUT /api/v1/bundles/{id}` rewrites `name` and `description` only. It publishes
no version, runs no cascade, and changes nobody's access — a bundle's name is
what operators call it, not what it grants. A name colliding with another
bundle's is `409`, not `500`.

`DELETE /api/v1/bundles/{id}` deletes the bundle and enqueues, per holder, the
closure diff computed with that bundle excluded — in one transaction with the
delete. Every table hanging off a bundle cascades on delete, so an assignment
row that vanished without its revoke would leave the role in Zitadel with
nothing in MkAuth explaining it. That is drift, arriving with no actor.

The welcome flag is REPORTED, never guarded against. Refusing to delete the
welcome bundle would be a rule an operator could not satisfy — the flag is
cleared only by promoting another bundle, which an organisation with one bundle
cannot do. The response carries `was_welcome` and the console states the
consequence before the click.

`onboarding_triggers.bundle_id` MUST NOT hold a foreign key to `bundles`. The
row records that somebody was onboarded and what they were given; that stays
true after the bundle is retired, and both alternatives corrupt it — a foreign
key makes every bundle that ever onboarded anybody undeletable, and
`ON DELETE SET NULL` rewrites the row to say they were given nothing.

#### Scenario: Holders lose different things
- **WHEN** a bundle carrying `Laser Lab / trained` is deleted
- **AND** one holder also has a direct grant for that role and another does not
- **THEN** exactly one revoke is enqueued, for the holder with no other source

#### Scenario: The welcome bundle is deleted
- **WHEN** the bundle carrying `is_welcome` is deleted
- **THEN** the response reports `was_welcome: true`
- **AND** the console states that onboarding will grant nothing until another bundle is set

#### Scenario: Nobody holds it
- **WHEN** a bundle with no holders is deleted
- **THEN** the bundle is deleted and no propagation rows are enqueued

### Requirement: A member MUST be able to withdraw their own pending request

`POST /api/v1/requests/{id}/withdraw` resolves a request to `withdrawn`. It is
user-gated and self-only: the authenticated subject MUST be the requester, and
the `UPDATE` is scoped by `requester_user_id` as well, so the guard does not
depend on the handler alone. Operators are not exempt — an operator taking back
somebody else's ask is a rejection with the reviewer's name left off, and the
decision route exists for that.

`withdrawn` is a resolution without a decision: `resolved_at` is set and
`reviewer_user_id` stays NULL, enforced by
`ck_access_requests_pending_unresolved`. The row already names who filed it;
recording them again as their own reviewer would state a fact the row states,
in a column that means something else.

Any status other than `pending` MUST be treated as settled by the decision path.
Enumerating the decided statuses instead would silently make each new terminal
state decidable again.

#### Scenario: The requester withdraws
- **WHEN** the person who filed a pending request withdraws it
- **THEN** its status becomes `withdrawn` with no reviewer recorded
- **AND** it leaves the operator queue and the pending-requests indicator

#### Scenario: Somebody else tries
- **WHEN** any principal other than the requester calls withdraw
- **THEN** the response is `403` and the row is unchanged

#### Scenario: An operator decided it first
- **WHEN** the request was approved between the member's page load and their click
- **THEN** the response is `409` naming the request as already decided

#### Scenario: A withdrawn request reaches the decision endpoint
- **WHEN** an operator approves or rejects a withdrawn request
- **THEN** the response is `409` naming its current state

### Requirement: A settled request MUST NOT be rendered as a decision nobody took

Both the operator queue and a member's own list resolve a status through one
function. `withdrawn` reads as "Withdrawn" to an operator and "You withdrew
this" to the member; it MUST NOT read as "Denied" or "Not approved" in either.
An unrecognised status is echoed back rather than bucketed — a status the
console has not been taught about must never render as an approval on a record
of who may operate a laser cutter.

#### Scenario: The console proxy permits the withdraw route for members
- **WHEN** a member posts to `/requests/{id}/withdraw` through the console proxy
- **THEN** the proxy forwards it, with no body of its own added
- **AND** a member posting to any other `/requests/{id}/…` route is refused before the backend

#### Scenario: A status the console does not know
- **WHEN** a request carries a status neither view enumerates
- **THEN** the status string is rendered as-is, in the unsettled tone

## REMOVED Requirements

### Requirement: Recent cascades feed

`GET /api/v1/propagations/cascades` returned applied cascade-originated outbox
rows, one row per write. Change history (`/propagations/cascade-groups`)
replaced it on screen with one entry per cascade, which is the readable unit —
a row per write is the same data with the causation removed. The endpoint, its
handler and `db.GetRecentCascades` are deleted; `models.CascadeSummary` stays as
the per-write shape inside a `CascadeGroup`.
