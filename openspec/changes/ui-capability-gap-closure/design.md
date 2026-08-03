# Design — UI Capability Gap Closure

## Decision 1 · A role reference is a pair, composed in the UI

### The problem, stated precisely

`(project_id, role_key)` is the identity of a role. `role_key` alone is not.
Every screen in MkAuth knew this and none of them agreed on how to say it.

Two abstractions existed for it and neither was reachable:

| Mechanism | Where | Call sites before this change |
|---|---|---|
| `<RoleName projectId roleKey>` | `components/names/RoleName.tsx` | 0 (13 before `d8676df`) |
| `formatRoleRef(projectId, roleKey, projects)` | `lib/format.ts` | 0 — only its own test |
| `CatalogRole.display_label` | composed in `services/roles.go` | 1, and that one printed the project twice |

Meanwhile nine surfaces hand-rolled `<ProjectName/> / <Mono>{key}</Mono>`, and
two named a role with no project at all.

### Why `display_label` could not be the answer

`display_label` was `"Printing Lab: admin"`, composed backend-side and attached
to `CatalogRole`. It is only present on rows that come from `GET /roles`.

Grant rows, audit entries, access requests, propagation rows and drift items all
carry `(project_id, role_key)` and no label. So the one field named as the
disambiguation capability could not disambiguate any of the surfaces where two
same-key roles actually collide. Worse, it is a trap: it works on `/roles`, so
the next person to reach for it builds on a foundation that does not generalise.

Removed. The capability moves to the UI, which can resolve any pair through the
name resolver it already runs.

### The shape

Two exports, one rule, split by register — not by screen.

**`<RoleRef projectId roleKey />`** — for rows.

```
Printing Lab / trained
             ^ resolved via <ProjectName>   ^ raw key, monospace
```

The project resolves to its human name because that is what staff recognise.
The role stays as its raw key in monospace because the key is what lands in the
token and what an operator matches against Zitadel. A table is *scanned for
identifiers*, so it shows one.

**`roleLabel(projectName, roleKey, roleDisplayName?)`** — for sentences.

```
"Kanishk now holds Printing Lab / Trained operator."
```

Prose is *read*, so it gets the role's human name; `laser_trained` is not a
word. It is a plain function rather than a hook so a toast handler, a dialog
lede and a warning banner can all call it without a resolver in scope.

The invariant both enforce: **the project is never absent.** `<RoleRef>` with
either half missing renders an em dash rather than half an identity — a bare
`admin` is worse than nothing, because it looks like an answer.

### Where the pair is established by structure instead

`<RoleRef>` is deliberately *not* used inside a container already scoped to one
project. Repeating the project there is a stutter, not a clarification:

- `/roles` — Project is the first column and never collapses.
- `/projects/{id}` and `/projects/{id}/roles/{key}` — the whole page is one project.
- `PersonAccess`, `MemberAccess` — roles are grouped under project cards.
- `Makerspace` "Where access lives" — the project is the trailing column.

`Makerspace` was the one place this was wrong: it rendered `display_label`
("Printing Lab: admin") in the name slot *and* `project_name` in the trailing
slot, printing the project twice on every row. It now renders the role alone,
the same way `/roles` does.

### Surfaces that were already correct, and stay as they are

`policies` and `RemovalDialog` build `"{Project} / {key}"` by hand. They are
correct pairs in the right register — a rule-authoring screen and a destructive
confirmation both want the exact key, not a friendly name. Rewriting them
through the helper would trade precision for uniformity. Left alone.

The audit listed `GrantDirectAccess`'s role `<Select>` as a fifth format. It
isn't a defect: the sibling select fixes the project, so `display_name · key`
inside it is unambiguous. Its *toast* was the defect — it fired
`` `${userName} now holds ${roleKey}` `` after the dialog closed, naming a role
with no project for a write that is meaningful only as a pair.

Similarly, `MemberAccess`'s role rows are fine — they sit under a project card.
Its *expiry warning* sits outside every card, and read "Operator runs out on
12 Aug" to a member who holds Operator in three projects.

Both now name the pair.

---

## Decision 2 · Where a filter lives is a UX question, not a plumbing one

Three screens gained filters in this change and they do not all filter the same
way. That is deliberate; the rule is the reader's experience, not symmetry.

**Event activity filters in the client.** The backend offers `?status=`, and it
is the wrong tool twice over: it covers webhook events only, so a filtered view
would be half server-filtered and half not, and it takes one exact status, so it
cannot express a bucket. The two tables do not share a vocabulary — a webhook
event is `processed`, an onboarding trigger is `completed`, and neither word is
what a reader has in mind. `outcomeOf()` maps both into *did it work / is it
still going / did it break / did we choose not to act*. The page also polls
every five seconds; a server-side filter would drop it into a loading state on
every pill press.

An unrecognised status returns itself rather than falling into `done`. A new
backend status appearing as a success on a forensic log is precisely the silence
this page exists to prevent.

**Drift triage filters on the server.** No polling, the endpoint already
supports it, and the ordering is computed server-side and must not be re-derived.
`useRowSelection` already prunes a selection to live ids, so narrowing the queue
cannot leave a bulk action aimed at rows that are no longer on screen.

`user_id` is accepted by the backend and deliberately not offered here: "select
everything else for this person" is already on every row, works from the row you
are looking at, and does not ask anyone to find a name among three hundred. A
select would be a worse version of a control that exists.

**Every filter says when it is the reason a list is empty.** "Nothing here" and
"nothing here that matches" are different facts, and on the drift queue the
difference is whether unexplained access exists somewhere nobody is looking.

## Decision 3 · A drain has five outcomes, and four of them were silent

`DrainResult` carries `applied`, `failed`, `requeued`, `errored`, `halted` +
`reason`. Only the first two were ever spoken.

The two that were missing are the two that mean *do this again*:

- `requeued` — a transient error; the row will be retried on the next pass.
- `errored` — the Zitadel outcome was decided but MkAuth could not write it
  down. The row stays `in_flight` and the next drain reclaims it. The write may
  well have landed; MkAuth does not know that it did.

Reporting only the terminal pair turns both into silence on an HTTP 200. An
operator who resumes a queue of eight and reads "0 applied, 0 failed" concludes
the queue is idle. `describeDrain()` returns `{tone, message, detail}` from the
whole result, and only a pass with nothing left to do returns `success`.

Halting is three distinct facts and gets three sentences: another drain holds
the lock, Zitadel is unreachable, or a row has exhausted its retry budget and
everything behind it is untouched.

## Decision 4 · The audit cursor is a tuple, because timestamps collide

`/audit` capped at 200 with no offset and no cursor, so anything older than the
most recent 200 mutations org-wide was unreachable.

Offset paging would be wrong here on a list that grows at the head, but the
sharper reason is that `created_at` is the **transaction** timestamp: a cascade
that writes eight audit rows in one transaction writes eight rows at the
identical instant. A `before=<created_at>` cursor would return the rest of that
batch forever, or skip it entirely.

So the cursor is `(created_at, id)` and the ordering is
`ORDER BY created_at DESC, id DESC` — the id tiebreak is not cosmetic, it is
what makes the cursor mean anything. `buildAuditQuery` is split out from
execution so the placeholder arithmetic can be tested without a database: a
mis-numbered `LIMIT` does not error, it returns the wrong page.

The response stays a bare array; the client sends `before_at` + `before_id` from
the last row it holds, and a short page is the end. Adding an envelope would
have been a breaking change to buy a fact the client can already see.

## Decision 5 · The member catalogue is projects, not applications

The audit read `A7` as "the `applications` slice of `/catalog` is fetched and
read by nobody". Re-checking the shape, `ApplicationCatalog` is a token consumer
— a claim name and a format type. Nobody requests an application. What a member
asks for is a role in a project, which is the `projects` slice.

So the catalogue lists every project and its roles, held ones marked rather than
hidden: a list that rearranges itself per person is harder to talk about, and
"you already have everything here" is often the most useful thing the page can
say about a space.

It sits under My access rather than behind a third nav item, because "what do I
have" and "what else is there" are the same question asked twice, and the
contrast between them is the point.

## Decision 6 · A deletion is the revoke half of an edit, not a new mechanism

Buckets B and C asked for three things nothing could undo: a mapping rule, a
bundle, and a request a member had changed their mind about.

The temptation with all three is to write a deletion path. None of them needed
one. Every cascade in this product already projects an *effective-role closure
delta* — what a person holds after the change, minus what they held before —
and a deletion is that same computation with one edge removed:

| Deleting | Is | Computed by |
|---|---|---|
| A mapping rule | `CascadeRuleUpdated` with no replacement edge | `rulesAfter = rulesBefore − old` |
| A bundle | `CascadeBundleRemovedFromUser`, run over every holder | `userBaseHoldingsExcludingBundle` per holder |

That is why neither needed a coverage check. "Does somebody keep this role
anyway?" is not a question the delete asks — a role still produced by another
bundle, rule or direct grant simply never leaves the `after` set. Writing the
check explicitly would have been re-deriving, worse, something the closure
already knows.

The bundle case is a loop over holders rather than one pass over the bundle's
role list, and that is load-bearing. Two people can hold the same bundle and
lose different roles by it going away, because one of them also has a direct
grant. Coverage is a property of a person.

### Why deletion and revocation share a transaction

Every table hanging off a bundle carries `ON DELETE CASCADE`, so the assignment
rows vanish the instant the bundle does. A holder whose assignment disappeared
without a revoke keeps the role in Zitadel with nothing in MkAuth left to
explain it — which is not a gap, it is drift, and it would arrive with no actor
and be found weeks later by the sweep. The same argument holds for a rule.

So both go through the existing `*AndEnqueue` shape: mutation, audit row and
outbox rows in one transaction, then `applyMode` on the source's own
`confirmation_mode`. A rule whose writes queued for confirmation queues the
writes that undo it.

## Decision 7 · The welcome flag is reported, not guarded

Deleting the welcome bundle stops onboarding granting anything. The obvious
protection — refuse it — is a rule an operator cannot satisfy: the flag is
cleared only by promoting a different bundle, and a makerspace with one bundle
has nothing to promote. A refusal you cannot act on is a trap wearing a
safeguard's clothes.

So the deletion goes through, `DELETE` reads the flag *before* the row goes
(afterwards there is nothing left to ask), the response carries `was_welcome`,
and the dialog says so before the click. The console warns twice — once in the
confirmation, once in a toast afterwards — because this is the one consequence
with no other home once the bundle is gone.

### The foreign key that had to go

`onboarding_triggers.bundle_id` referenced `bundles(id)` with no `ON DELETE`
clause, which would have made every bundle that ever onboarded anybody
undeletable. Both obvious fixes corrupt the log:

- `ON DELETE CASCADE` deletes evidence that somebody was onboarded.
- `ON DELETE SET NULL` rewrites the row to say they were given nothing.

The row's claim — "this person was onboarded, and this is the bundle they were
given" — stays true after the bundle is retired. So migration `000021` drops the
constraint and keeps the column, which is the shape this codebase already uses
for history: `audit_logs.resource_id` and
`pending_zitadel_propagations.source_ref` are both plain ids with no foreign
key, for exactly this reason. A log records what happened; it does not hold the
past open.

`<BundleName>` on the event log renders `a bundle since deleted` rather than its
default em dash there — "— assigned automatically" reads as nothing having been
assigned.

## Decision 8 · A withdrawal is a resolution, not a decision

`withdrawn` sets `resolved_at` and leaves `reviewer_user_id` NULL, enforced by
the CHECK constraint rather than by convention. Nobody reviewed it. The row
already names who filed it, so recording them a second time as their own
reviewer would state a fact the row states, in a column that means something
else — and the operator queue, which reads `reviewer_user_id` as "who decided
this", would then name a member as a decider.

Two things fell out of adding a third terminal state, and both were latent bugs
rather than new work:

- **The decision path enumerated the decided statuses** (`approved || rejected`)
  instead of testing `!= "pending"`. A withdrawn request would have stayed
  decidable, resurrecting an ask its author had taken back. Fixed at the shared
  guard, which is the only place every decision route passes through.
- **Both request views rendered "settled and not approved" as a denial.** The
  first withdrawal would have shown a member's own retraction back to the
  operator as a refusal somebody made. `requestOutcome()` now gives one reading
  in two registers, and echoes an unrecognised status back rather than bucketing
  it — the same rule `outcomeOf` follows for event activity, and for the same
  reason.

## Decision 9 · The vault belongs to the person, not to the System page

The design brief filed shadow credentials under `S10 · System › Hardware sync`.
That cannot work: all four user-facing endpoints are self-only, so an operator
standing on a System page can only ever set their own credential — which is not
what anybody goes to a System page to do.

The surface is on Member · My access, last on the page. That screen answers
"what can I use, and why"; a password is a setting, not an answer, and putting a
password field above somebody's access would make the page read as an account
screen.

Two sentences on that card are non-negotiable, and both are about what it is
NOT. It is not the institutional login — a second password field with no
explanation invites exactly one reading, and it is the wrong one. And nothing
reads it yet: the hardware bridge is unbuilt, so a password set today is stored
and waiting. Leaving the second out would be worse than leaving out the whole
card — somebody sets one, tries a door, and concludes the product is broken.

The dialog does not re-implement the complexity rules. `ValidatePasswordComplexity`
already composes the failing requirements into one sentence, and that sentence
is rendered verbatim. One authority on what counts as strong enough, and it is
the one that decides.

## Decision 10 · `/propagations/cascades` is deleted, not exposed

The audit's B2 asked to expose it or delete it. Deleted.

Change history replaced it correctly — one entry per cascade rather than one row
per write — and a row per write is the same data with the causation removed.
`models.CascadeSummary` survives as the per-write shape *inside* a
`CascadeGroup`, which is where a write is readable: as part of the event that
produced it.

## Decision 11 · The console proxy is a second lock, and it fails silently

Both new member-facing capabilities — `C3`'s withdraw and `B1`'s vault — shipped
correct on the backend and dead in the browser. The console proxy carries its own
member allowlist, and neither route was on it. Nothing failed loudly: the
withdraw button 403'd, and the vault card simply *was not there*, because it
suppressed its own read error to avoid a broken-looking box on somebody's home
screen.

Three consequences, all now closed:

**The allowlist is where a member-facing route becomes real.** The backend's
self-only enforcement is the inner lock and it was never the problem. A route
guarded correctly on the inside is still unreachable if the outer list has not
been told about it, and the symptom is indistinguishable from "not built".

**A blanket rule stopped being safe the moment a second write existed.** The
proxy stamped `requester_id` onto every member `POST`/`PUT` body. That was
harmless only while filing a request was the sole thing a member could write —
`decodeJSONStrict` rejects unknown fields, so the vault's `PUT {password}` would
have arrived with a field nobody sent and returned 400 on a correct password.
The injection is now scoped to the one route that carries a requester. Neither
review finding named this; it was found by reading the proxy after they did.

**Suppressing an error to keep a screen tidy hides faults.** `ShadowCredential`
returned `null` on a failed status read. It now renders an unavailable state
saying access is unaffected. A member cannot act on that message, but somebody
they mention it to can — and an absent card reports nothing to anybody.

The proxy's test file gained cases for each: they fail against the pre-fix
route (verified by stashing it), and one of them had to be tightened first — an
assertion on `calls[0]?.body` passes vacuously when the proxy refused the call
and there is no `calls[0]`.

## Decision 12 · The audit row's cascade id is stamped by the code that mints it

C6 was described as a threading problem: carry a cascade id onto every
`INSERT INTO audit_logs`. Reading the eleven call sites turned it into a smaller
and better one.

`enqueueCascadeRows` already minted one id per batch — that is how Change history
groups the writes one event produced — and then **discarded** it. Every atomic
`*AndEnqueue` function wrote its own audit row on the line immediately above its
call to that function. So the two halves of one event were written a line apart,
by two statements, with the identifier tying them together known only to the
second.

Threading the id outward would have made "the audit row names its cascade" a
convention eleven functions had to keep, and a twelfth would not have known
about. Moving the audit insert *inward* makes it structural: `enqueueCascadeRows`
takes the audit rows as a parameter, mints the id once, and stamps both. Eleven
pairs of statements collapsed to eleven calls, and a cascade added later cannot
forget, because there is nowhere left to forget it.

`MoveHoldersAndEnqueue` is why the parameter is a slice rather than one row:
moving eight people onto a version is eight things that happened to eight
people, and one thing that happened. All eight audit rows carry the same
cascade id.

**A cascade that reached nobody gets NULL.** Change history is built from the
outbox, so an id with no rows behind it would render as a link to a page with
nothing on it. "This change reached nobody" is better said by a blank.

## Decision 13 · Old audit rows keep their object, and lose the lie

Every row written before migration `000023` has no cascade id, and there is no
honest way to give it one. Matching audit rows to outbox batches by timestamp
proximity would be mostly right — a cascade writes its audit row and its outbox
rows in one transaction, at one instant — and *mostly right* is the wrong
standard for a lineage link on a record of who may operate a laser cutter.

They keep what they do know. `traceFor` returns three shapes and the console
renders each differently:

- **cascade** — the real id, linked to that cascade and no other.
- **object** — the bundle or rule the change was about, labelled `b_` or `R_`
  (the same vocabulary Change history uses), with **no link**. This is the same
  identifier the column showed before, minus the `c_` prefix that misdescribed
  it and the link that went somewhere else.
- **none** — a dash. `bundle.role_added` records its resource as
  `project/role`, and the first four characters of that is a label that looks
  like a handle and refers to nothing.

The old column was one function of the *action name*; the new one is a function
of what the row actually carries. Both audit surfaces render it through one
component for the same reason they share an action vocabulary: an operator
comparing two screens must not find one row tracing to two different things.

`?cascade=` is answered by the query, not by filtering the fifty-entry glance
list in the console. The audit tail is walkable back to the first day, so a
trace from an event older than the fifty most recent cascades has to land on
something — and when the outbox rows have been drained and cleared, the page
says *that*, rather than "nothing has cascaded yet".

## Decision 14 · An application lives in exactly one project

C7 (ISC-45) is settled, and settled the way the schema already was. Zitadel puts
an app inside one project; `app_claim_overrides.application_id` is UNIQUE. The
design diagram showing one Badge Reader reading four projects was the only thing
claiming otherwise, and a diagram is the cheaper of the two to correct.

The alternative was a real feature, not a relaxation: a join table, a rule for
competing claim overrides across projects, a token-audience decision, and a
deletion rule for when one of an app's projects is removed. None of those
questions has a caller. Building the data model to answer them in advance would
be building it for a use nobody has.

The apps index keeps warning per project, which is now correct rather than a
workaround. **What would reopen this:** a real integration that needs roles from
two projects in one token. That is the trigger, and it is a product event, not a
refactor.

## Decision 15 · Advanced shows Zitadel's grant id, not a second MkAuth one

C9's buildable half. The Advanced panel already showed `direct_role_grants.id` —
MkAuth's own row — which answers "what does MkAuth think" and not "what does
Zitadel hold". The second is the one an operator needs when cross-checking the
identity provider or quoting an id in a ticket.

It is keyed by **project**, because that is Zitadel's shape: one user-grant
carries every role the person holds in that project. Repeating it on each role
row would imply each role has its own grant, which is the misreading this whole
page exists to prevent.

Operator-only and Advanced-only, and *not fetched* otherwise: the endpoint
behind it is operator-gated, this route also serves a member their own record,
and a member's page must not fire a request whose only outcome is a 403 (the
same rule as the Activity tab).

A project with no Zitadel grant says so, rather than showing a dash — MkAuth
listing roles for a project Zitadel has no grant for is a real condition, and a
dash reads as "not loaded". Naming it is not the same as interpreting it: the
line points at Reconciliation, which is where that gets triaged, and makes no
claim of its own.

**C9's other half stays unbuilt.** There is no per-user hardware sync state to
render while the bridge is parked, and a panel that invented one would be
precisely the failure `/system/hardware-sync` was written to avoid.

## Decision 16 · A cascade id is a handle into one screen, so only that screen's writes get one

Caught in review. Decision 12 gated the audit stamp on `len(params) > 0`, which
read as "did this event cause any writes". The right question is narrower: *will
those writes appear on the screen this id links to.*

`DeleteDirectGrantAndEnqueue` goes through `enqueueCascadeRows` because its
ledger delete, audit row and outbox rows must commit together — not because it is
a cascade. Its writes carry `source='direct'`, and `GetCascadeGroups` filters to
`bundle | rule | lifecycle_cascade`. So a direct removal stamped an id, the audit
column rendered it as a trace link, and the link opened a page whose query
excluded the very write it was about. Worse than a dash, and worse than a blank
page: the empty state I had written says *"that cascade is no longer in the
queue — the writes it produced have been carried out and cleared"*, which is a
confident false statement about a revoke that was still pending.

The fix is that the stamp and the filter now read **one list**,
`cascadeGroupSources`, passed to the query as a parameter instead of spelled out
inside it — twice, as it had been. A source added to the cascade family updates
the glance list, the `?cascade=` lookup and the audit stamp in one edit, and a
guard fails if the query ever inlines its own copy again.

Two consequences worth stating rather than leaving implicit:

**A direct removal that fires a rule is still not a cascade.** Removing a direct
grant can revoke a second role a mapping rule derived from it. `deltaParams`
attributes the whole delta to the grant, because the grant is what an operator
clicked, and Pending changes is where an operator's own writes are answered for.

**The predicate is asserted across the package boundary.** The params are built
in `services` and read in `db`, and the bug lived in the gap between them, so
`db.IsCascadeGroupSource` is exported and `services`' direct-removal test asserts
against it directly. Testing either package alone would have passed.

## Decision 17 · The action vocabulary is checked against the backend, not against memory

Also caught in review, and the same shape of failure: `describeAction` falls
through to the raw action key for anything it does not recognise. That fallback is
correct — a log that invents a description is worse than one admitting it does not
know — and it is **silent**, so six actions had accumulated behind it:
`bundle.updated`, `bundle.deleted`, `bundle.version_published`,
`bundle.holder_moved`, `mapping_rule.deleted`, `access_request.withdrawn`. Four
came from this change; two arrived with bundle versioning and had been rendering
as `bundle.version_published` on screen ever since.

A hand-maintained list in a test would have the same weakness as the map. The
coverage test reads the Go sources instead, collecting dotted literals from lines
that name an `Action:` field or mention audit at all, and fails naming the action
plus the file and line that writes it. It asserts in both directions — an action
with no sentence, and copy for an action nothing emits.

It cannot see the two actions assembled by concatenation
(`"access_request."+status`, `"direct_grant."+opTypeAuditVerb(...)`). Both
families are covered, and a scanner that evaluated Go string arithmetic would be
a worse thing to own than the comment saying so.

The wording choices are deliberate in two places. `bundle.updated` is *"Changed a
bundle's name or description"*, not *"Renamed"* — the endpoint rewrites both and
the row records neither, and a specific falsehood is worse than a vague truth.
`access_request.withdrawn` is *"Withdrew their request"*, never a refusal, for
the same reason `requestOutcome` keeps that distinction on the request screens.

## Decision 18 · The reopen rule is a stored value and a comparison, not an invalidation

C4 is built, with the rule you chose: an acknowledgement ends when the grant
changes.

The obvious implementation is a boolean plus something that clears it — a trigger
on `direct_role_grants`, or a check in every write path that touches `expires_at`.
Both are the same mistake in different clothes: the rule would live somewhere
other than where it is read, and the failure mode is silence. Miss one write path
and an acknowledgement quietly becomes permanent, which is precisely the rule we
rejected.

So the acknowledgement stores **what was acknowledged** —
`acknowledged_expires_at` — and the read returns it only while it still equals the
grant's current `expires_at`. That single join condition *is* the rule. Nothing
fires, nothing has to remember, and there is no second copy of the truth to keep
coherent. `direct_role_grants` is UNIQUE on `(user, project, role)` and upserts in
place, so extending keeps the grant id and moves the date, which is exactly the
case the comparison catches.

It also makes the rule testable in a package with no live database. Two source
guards — the column exists and is NOT NULL, and the join compares the dates — hold
the whole behaviour. A trigger-based version would have been unverifiable here.

**Consequence accepted:** stale acknowledgement rows are not deleted. They are
inert, because validity is a comparison. If a date is moved and later moved back
to the original day, the old acknowledgement applies again. That is defensible on
its own terms — the operator agreed to "let this lapse on the 1st", and it lapses
on the 1st — and the alternative is a sweep whose only job is tidiness.

**The write is checked, not trusted.** `expires_at` is required on the request and
compared to the row under `FOR UPDATE`. A stale page gets `409` and told to
reload. Storing the acknowledgement anyway would be worse than refusing it: it
would be accepted, never apply, and leave somebody believing they had recorded a
decision — the exact failure the old "no second button" copy was written to
prevent.

**`ON DELETE CASCADE`, reversing the reasoning of 000021 and 000023.** Those
argued against foreign keys because audit rows and onboarding triggers are
*history* and must outlive their subject. An acknowledgement is not history; it is
an annotation on a live row, meaningless once the grant is gone, and the grant
being swept away on its date is the normal end of its life. The history of who
decided what is in `audit_logs`, where it belongs and where it stays.

## Decision 19 · Acknowledging is never bulk, and never hides a row

Two restraints on a feature whose whole purpose is to reduce work.

**Per-row only.** The record's entire value is that a person read the row. A
checkbox and an "Acknowledge selected" would let twelve rows be silenced in one
gesture, which produces the appearance of review and none of it. Bulk *extend*
stays, because extending changes access and reviewing a dozen of those together is
the work this screen was built for. Eight clicks for eight decisions is the
correct price.

**Grouped, not hidden.** Acknowledged rows move below a counted heading. Removing
them would be the client-side dismissal §S7 forbade — and it would hide a decision
from the person who made it, who is the one most likely to want to revise it. A
heading is where the eye stops; a filter would be a control an operator has to
remember to check.

Three smaller calls in the same spirit:

- **Extend stays on an acknowledged row**, and "Let it lapse" becomes "Undo".
  Changing one's mind toward *keeping* somebody's access must never be harder than
  the decision that lets it go.
- **No checkbox on an acknowledged row**, so a bulk extend cannot reverse a
  decision by accident.
- **A dialog, not a click.** The note is what makes the record useful to the next
  operator rather than to the one writing it, and the dialog is the only place to
  say the two counter-intuitive things: that this does not keep the access, and
  that it reopens if the grant changes. Neither is discoverable by trying it.

The old copy explaining why there was no second button is deleted rather than
softened. It said such a control "would submit nothing and change nothing", and
that is now false — leaving it would have made the screen argue against itself.

## Decision 20 · A bulk write is scoped by what the screen's rows ARE

Caught in review, and older than C4 — the acknowledgement work only made it
easier to see.

Review › Expiring access has always put its checkbox on a **grant**. The bulk
contract has always been keyed on **people**. The screen bridged the two by
reducing its ticked rows to user ids, so ticking "Ada / Laser Lab / trained"
extended every expiring direct grant Ada held: other projects, and dates months
past the 30 days the screen is scoped to. Rows the queue had never rendered, on a
record of who may operate machinery.

Two remedies were available. Making the UI select *people* and disclose the full
blast radius would fix the honesty and lose the capability — the operator would no
longer be able to renew one role and let another lapse, which is the ordinary case
on a per-grant queue. So instead the contract now carries what was ticked:
`grant_ids` on `extend`, narrowing the write to exactly those.

Omitting it keeps the other meaning, which is also correct. On People the rows are
people, and "extend their expiring access" is precisely what selecting them asks
for. **The rule is that a screen whose rows are grants must pass grant ids**, and
`grant_ids` is refused on every other op rather than ignored — accepted silently,
it would let a caller believe they had scoped an operation they had not.

The id set is flat across the selection and applied per person. A grant id belongs
to exactly one person, so one person's tick cannot reach another's access; a
foreign id simply matches nothing. A person with nothing selected reports "none of
the selected grants are theirs any more", which is a different fact from "no
expiring direct grants" and the difference matters to somebody reading a plan
before authorising it.

## Decision 21 · Every write that moves an expiry must drop the queue built from expiries

The other half of the same review, and one caller wider than reported.

`POST /users/{id}/grants` upserts on `(user, project, role)`, which makes it both
"grant access" and "extend access" — Review › Expiring access uses it for the
second. Its invalidations covered the person and the user list, and not the queue
the operator was standing in. After extending, the row stayed on screen with its
old date; if it had been acknowledged, it also kept showing an acknowledgement the
new date had already voided. The screen would have been contradicting the backend
about who keeps access — and by exactly the mechanism the acknowledgement was
designed to make trustworthy.

`useApplyBulk` had the same hole, unreported: its key list named `users`, `roles`,
`bundles`, `governance`, `propagations` and `audit` — every root except the one
belonging to the screen a bulk extend is launched from. A blanket list is only
blanket until somebody adds a surface.

Both now invalidate `review`, and `useCreateGrant` also invalidates `governance`,
because Today counts what is expiring and was wrong by one until the next refetch.
Tested by asserting the key roots rather than the rendered result: the bug was in
the mutation's contract, not in any one screen, and asserting it there is what
makes it hold for the next screen too.
