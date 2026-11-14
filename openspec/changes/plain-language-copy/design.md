# Plain-language copy — the writing guide

Every sentence Syndra shows is read by someone who runs a makerspace, or by
someone who wants to use one. Neither has studied identity management, and
neither should have to. This document is the rule for all text in the product:
what it is for, how it sounds, which words it uses, and what every page,
action and message must contain.

It is a contract, not advice. `ui/src/components/ui/__tests__/plain-language.test.ts`
checks the parts of it a machine can check. The rest is checked in review by
reading the sentence aloud to somebody who does not work here.

---

## 1. Who is reading

**Makerspace staff.** They run an academic makerspace at a university. They
know what an identity provider is, they can name every product in their own
rack, and they configured most of it. Write for a competent colleague: they
do not need *token* explained, and a sentence that explains it anyway costs
them a clause and tells them you assumed otherwise.

What they cannot know from experience is the vocabulary **this product
invented** — a cascade, the outbox, a merge finding, a log anchor, what
Syndra means by adopting an account. Those are not difficult, they are
simply local, and the only place they are ever explained is here.

**Members.** Students, faculty, visiting researchers. They come to see what
they can use, to ask for more, and to reach storage. They may open Syndra
twice a term, and they are not administrators of anything. They get the plain
register — but they get the real names of the systems they actually use, TrueNAS
and Zitadel among them, because a student who cannot mount a share is going to
type the product's name into a search box, not "the network storage server".

**The person who runs the server.** One or two people. Text for them is
marked as theirs — *for whoever runs the Syndra server* — so everyone else
knows the paragraph is not.

The register differs by audience. The **facts** never do, and neither does
the vocabulary: one word per thing, for everybody.

## 2. Register

Syndra belongs to a college. Its voice is the voice of a well-run university
office: courteous, unhurried, exact. It respects the reader's time and their
intelligence, and it never performs either.

- **Calm.** No exclamation marks. No urgency the situation does not have. A
  queue of forty is "forty changes waiting", not "40 changes need your
  attention now!"
- **Courteous, not servile.** No "please", no "sorry", no "oops". Courtesy is
  shown by being clear, by saying what will happen before it happens, and by
  never blaming the reader for what the software did.
- **Exact.** A number has a unit and a referent. A time says when. A
  consequence names the person it falls on. "Some of these may not apply" is
  not a sentence Syndra writes; "3 of the 12 already hold this role and will
  not change" is.
- **Plain, but not simplified.** Short words where a short word is as exact —
  never a longer paraphrase in place of the right term. "The changes this edit
  set off" is not plainer than "cascade", it is vaguer and four words longer,
  and it leaves the reader without the word the rest of the product uses.
  Precision is the courtesy; §6 says where a term carries its definition with
  it. The reader should never feel a sentence was written to sound
  authoritative, nor that it doubted them.
- **Never clever.** No wordplay, no arch asides, no jokes. Cleverness costs the
  reader a second reading and gives them nothing for it.
- **Never lecturing.** Design rationale belongs in code comments and in this
  document, not on screen. A sentence that begins "This section keeps its
  seat so that…" is talking to the developer.
- **Second person, present tense.** "You can revoke this" rather than "The
  operator may revoke this". "Syndra sends the change" rather than "the change
  will be dispatched".
- **Whole sentences.** A label may be a noun phrase. Anything longer is a
  sentence with a full stop, and reads as one.

### The keeper's voice

Syndra speaks as the door-keeper who knows everyone (A-18, owner-accepted 5 Nov 2026):

- She uses your name and knows the time of day.
- She speaks up only when something has changed for you.
- When everything is in order, she says so in one short sentence and stops.
- She uses the door and the list only where they are literally true. Drift really is "someone inside who isn't on the list". She never uses them to decorate.
- She doesn't joke, doesn't exclaim, and never speaks in a confirm, an error that needs action, or a log.

The character is in *when* she speaks and *how exactly*, not in extra words. Strings added by A-18:

| Where | Words |
|---|---|
| My access, calm | "Nothing needs you right now." (the same words as Home) |
| My access, greeting | "Morning, Cody. Here's what you can use." (Morning before 12, Afternoon before 17, Evening after) |
| My access, quiet month | "Nothing has changed in the last 30 days." |
| My access, access starts | "From today, you can use Laser cutting." |
| Drift, all clear | "Everyone inside is on the list, and everyone on the list is inside." |
| Unfinished revocations, all clear | "Everyone whose access was revoked is out." |
| Home's date line, a term's first or last day | "Last day of Monsoon term", "First day of Spring term" |
| Account sheet, About Syndra | the story of the name, closed until opened |

## 3. Shape of a sentence

1. **Consequence before mechanism.** Say what happens to the person first;
   say how Syndra does it only if the reader needs it to decide.
   *Not:* "The row is enqueued to the outbox and dispatched on the next drain."
   *But:* "Their access ends within about a minute."
2. **Name who acts.** "Syndra creates the account." "You decide." "TrueNAS
   refused it." Passive voice hides the actor, and the actor is usually the
   thing the reader most wants to know.
3. **One idea per sentence.** Two consequences are two sentences.
4. **No stacked negatives.** "Cannot be undone" is one negative and fine.
   "Not unless nobody has not…" is not written.
5. **Every pronoun has a visible referent.** "It" and "this" refer to a noun
   in the same sentence or the one before. When in doubt, repeat the noun.
6. **Say what did *not* happen.** Every refusal and failure ends with the
   state of the world: "Nothing was changed." Every partial outcome says
   which part. The reader must never have to guess whether to try again.
7. **Say what to do next**, when there is something to do — and say where.
   "Ask makerspace staff" is a next step. "Contact an administrator" is not.
8. **Numbers carry their unit and their whole.** "3 of 12 people", "2.4 GB",
   "in 14 days", never a bare "3".
9. **Times are absolute and relative together** where the gap matters:
   "12 Sep (in 14 days)". A relative time alone rots; an absolute one alone
   makes the reader do arithmetic.
10. **A system is named by its name.** TrueNAS, Zitadel, Google. Never "the
    target", "the provider", "upstream". If the name is not known, say "the
    connected system".

## 4. What every page carries

Every page has a **title** and a **lede** — one or two sentences under the
title that answer, in this order:

1. what this page shows,
2. when you would come here,
3. for a queue or review list: what happens if you do nothing.

The third is the sentence most often missing and most often needed. Ignoring
*Expiring access* ends someone's access; ignoring *Holds due* keeps it
blocked; ignoring *Pending changes* means nothing is sent. Each of those pages
says so in its lede, in the same breath as what the page is.

The lede lives in `PageHeader`'s `lede` prop. `meta` is for metadata — a
count, an email, an identifier — and never for a sentence. A page with no lede
fails the guard, unless its first content already answers the lede's
questions better, argued in the guard's `LEDELESS` list. One page is there: an
app's page, whose summary says what the app gets (A-15 §1, owner, 3 Nov).

Every **section** has a heading of five words or fewer that names the thing
listed, not the mechanism that produced it. "Not created by Syndra", not
"Unvouched bindings".

Every **empty state** says what would put something here. "No requests
waiting. Requests appear here when a member asks for access." Never "Nothing
here" alone.

Every **term this document lists as glossed** is glossed on its first
appearance on the page, in a parenthesis or a short following clause, in the
words section 6 gives. Once per page, not once per product: a member who
opens *Network storage* has not read *Bundles*.

## 5. What every action carries

**The button names its object.** "Revoke this role", "Send 12 changes",
"Approve for Cody". Never "OK", "Go", "Submit", "Confirm" alone. Out of
context — read by a screen reader from a list of buttons — the label must
still say what it does.

**The consequence comes before the click.** A line above or beside every
button that changes something, stating what will be true afterwards and for
whom. Destructive actions get a short titled list, *What happens*, with one
bullet per consequence and the alternative named beneath it.

**Confirmation follows the three rungs** (`one-control-surface`), and the copy
of each rung is fixed:

- *Rung 1* — the button carries the number: "Apply to 12 people"; a plan
  that cancels waiting changes carries that too, in the cancel step's shape:
  "Cancel 2 and apply to 12 people" (A-21 §2.5).
- *Rung 2* — a sentence to tick, with the number inside it:
  "I understand this revokes access for 12 people."
- *Rung 3* — type the name. The label says what unlocks: "Type **Laser
  Cutter** to confirm. The button below unlocks when it matches."

**A preview is called a preview.** Every dialog that previews before applying
opens with the same sentence: *"Syndra first shows exactly what would change,
person by person. Nothing changes until you press Apply."*

**The outcome is a sentence, not a status.** What happened, to how many, and
what did not. "Applied to 9 people. 3 already held this role and were left as
they were." A failure ends with "Nothing was changed." A partial outcome names
both halves.

**A queued outcome says where it went and what moves it.** "Recorded in
Syndra and waiting to be sent to Zitadel. Nothing has changed there yet; send
it from *Pending changes*."

**A change the system didn't take says when it goes again** (owner, 5 Nov
2026). One retry model, one sentence, wherever it was sent and on its row:
"Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6)." The time
is in the makerspace's zone. It is queued, never "Sent", and carries no "send
it" link: Syndra sends it on its own, unless the role waits for you. When the
tries run out, or the system refuses a try: "Syndra stopped sending it", listed on Pending changes under *Syndra stopped sending*. A member's own password says
"Not confirmed yet. Syndra is trying again at 14:05. Keep this page open." The
words live in `ui/src/lib/retry.ts` and nowhere else.

**An outcome always says the next step, or that there is none** (owner, 5 Nov
2026: "along with clear communication, there should be clear next steps or
lack thereof"). Every outcome, state and notice ends on one of two things: what
to do and where, as a link or button when there is one ("Press Send on Pending
changes", "Check TrueNAS under System", "Ask Cody to sign in once"), or an
explicit *Nothing for you to do.* ("Nothing for you to do. Syndra sends it
again at 14:05 and says on Home if it stops sending it."). Silence is never the "none".
It holds by construction: `nextStepOf` (`lib/outcome.ts`) gives every outcome
kind its step and `ActionOutcome` draws it in every placement; `retryNextStep`
(`lib/retry.ts`) gives every retry state its step and `RetryStatus` draws it;
`ErrorState` ends on `nextStep`. A caller who knows better sets `next`. Guard:
`lib/__tests__/next-step.test.tsx` fails on any kind or state with no step.

| State | Word | Next step, or none |
|---|---|---|
| Sent, confirmed | Done / the act's word | Nothing for you to do. |
| Waits for you (a role marked to wait) | Waiting for you to send → | Press Send on Pending changes (the link). |
| Retrying | Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6). | Nothing for you to do. Syndra sends it again at 14:05 and says on Home if it stops sending it. |
| Not taken, and this server sends nothing again on its own (`REVOCATION_DRAIN_ENABLED=false`) | Zitadel didn't answer after one try, so Syndra won't send it again on its own. | Press Send on Pending changes to try again. |
| Stopped sending (out of tries, or refused) | Syndra stopped sending it. | Give a role again from the person's page; a revocation shows in Drift after the next check, to revoke there. Then Mark seen on Home. |
| Sends itself (a revocation) | On its way | Nothing for you to do. Syndra sends it on its own and says on Home if it stops sending it. |
| A role's standing, per person (`lib/standing.ts`, one-path W3b; the backend works it out, screens only render it) | Operator: held In Zitadel; sent Sent, not seen back yet; waiting Not sent to Zitadel yet (with Given … ago and Waiting for you to send); removing Being removed; not held Not in Zitadel; not known Couldn't check Zitadel. Member: held silence; sent and waiting On its way; removing Being removed; not held Not there yet; not known Couldn't check. | Waiting: Press Send on Pending changes (operator). Sent and removing: nothing for you to do. Not held: Decide on Drift. |
| May have gone through | Some of this may have gone through. | Check *where* before trying again. |
| Failed | Failed | Try again in a minute. If it keeps failing, ask the person who runs the Syndra server. |
| Refused | Refused | What refused it says the step: sign in again, ask someone with access, reload, or change what it names. |
| A member's own password, retrying | Not confirmed yet. Syndra is trying again at 14:05. | Keep this page open. It says here when it's done. |

**Retries are seen in both views.** While Syndra is sending changes again on its
own, Home says so in *Sent again on its own* and the rail carries a pill beside
Home (and beside Pending changes in Advanced): amber "↻ 2" while retrying, danger
"! 1" once it gave up. A change Syndra stopped sending stays until you press **Mark seen**; it is yours alone, so each person marks their own (migration 000082).

**Every Notice ends on a next step or none** (6 Nov): `Notice` takes `next` (what to do, or where to go) or `nothingToDo` (why there is nothing; drawn as "Nothing for you to do." plus it). One is required by its types.
Retrying changes are never counted as "waiting for you to send".

**While Zitadel isn't answering, the line before the act is exact per act**
(`useWaitsBeforeLine(op)`): "Zitadel isn't answering. Syndra records the access
now and sends it on its own once Zitadel answers, so there is nothing for you
to send. A role marked to wait still waits for you to send it." (the removal,
the change). Never "will wait for you to send": a removal goes on its own, and a
role given while Zitadel is down is retried on its own.

**The action signing key's states** (`app/zitadel/page.tsx` `keyState`,
`SigningKeyGuide.tsx`):

| Word | Means | Next step |
|---|---|---|
| Within its limit | Replaced in time | Nothing for you to do. Syndra replaces it on its own before it is due. |
| Replacing | Zitadel is making a new key | Nothing for you to do. |
| Not confirmed | Replaced, the test sign-in didn't confirm it | Claims delivery says what the test sign-in got. |
| Partly replaced | Zitadel replaced one key but not the other; the old one still works for the other | Replace now again to finish, or do it by hand. |
| Not known | Zitadel didn't answer and the read-back failed, so nobody can tell; the old key keeps working | Check the row in a minute. If it still says Not known, press Replace now. |
| Not replaced | Zitadel refused; the old key still works | Syndra tries again on its own at the named time, or Replace now. |
| Past its limit | Overdue | Replace now, or Syndra does it in the next quiet hours. |

**Background timing is said as it is.** The revocation and retry runner ticks
every minute, so a revocation "sends on its own, within a minute"; Zitadel's
setup is checked "when Syndra starts and every 10 minutes after". Never "every
few minutes" for either.

**The first 100ms is said in the future tense.** While a write is in flight the
seat says what Syndra *will* do, never that it did (A-19 P2). The forecast is a
sentence about Syndra's own next step, never a claim about the target, and it
carries no number. When the server answers it crosses to the true word. The
words are `FORECAST` in `lib/outcome.ts`; a test holds this table to it.

| Act | While it is in flight | When it has happened |
|---|---|---|
| Approve | Will be approved | Approved |
| Decline | Will be declined | Declined |
| Extend | Will be extended | Extended |
| Remove (a bundle out of a person's set) | Will be removed | Removed |
| Revoke direct access | Will be revoked | Revoked (A-22 Q4) |
| Revoke under Drift | Will be revoked | Taken: Removed in Zitadel. Not taken yet: the retry line ("Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6)."). Halted with the drain on: On its way, Revocations send on their own. Drain off: Recorded in Syndra. The revocation waits in Pending changes until someone presses Send. Drain not read yet: Not in Zitadel yet, then "Open Pending changes to see what has not reached Zitadel yet." A role that waits: Waits for you, Waiting for you to send. Refused: Refused by Zitadel. Applied: Revoked |
| Apply bundle changes (Manage bundles) | Will be applied | Recorded; failure Not applied |
| Keep | Will be kept | Kept |
| Lift a hold | Will be lifted | Lifted |
| Give | Will be given | Given |
| Cancel a queued change | Will be cancelled | Cancelled |
| Adopt | Will be adopted | Adopted |
| Mark as owned elsewhere | Will be marked | Marked |
| Replace the action signing key | Will be replaced | Replaced |
| Anything Syndra knows will wait | Will wait for you to send | Waiting for you to send |

**Anything Zitadel refused outright** has no forecast of its own (the act's own forecast stands) and, once it happened, reads Refused by Zitadel: "{Done} in Syndra. Zitadel refused this: {cause}." Then "It isn't in Zitadel, and Syndra won't send it again. Pending changes lists it under Syndra stopped sending." The step: give or extend it again from their page once the reason is fixed; a revocation shows in Drift after the next check, to revoke there. Never "Nothing was changed.": Syndra recorded it.

**A refusal's cause is said in plain words** (`refused:<cause>`, `lib/wait-reason.ts refusalCause`; a plan row's seat under "Zitadel refused" is `lib/plan-groups.ts refusedSeat`, "Zitadel refused this: {cause}. {step}", under the row's own plan words): not allowed, "Syndra's service account isn't allowed to make this change" (and the step starts "Ask whoever runs Zitadel to let Syndra's service account change grants."); not found, "it no longer has this person, project or role"; not valid, "it said the change isn't valid"; anything else, "it gave a reason Syndra can't put in plain words".

**A single act that would cancel a waiting change says so before the press**
(A-21 §3, architecture deepening 1.2). The sheet reads the act's plan as soon
as it has enough to plan, and its before-line says it: "Giving it cancels the
removal Randy Orton queued 2h ago. Nothing is cancelled until you choose on the
next step." became, with the owner's 8 Nov rule that give is only a verb,
"Giving it cancels removing Member in Laser Lab from Randy, which was waiting for
you to send." (several: "Giving it cancels 2 changes that are waiting.");
"Revoking it cancels giving Member in Laser Lab to Randy, which was waiting for
you to send.", "Approving cancels removing … from …, which was waiting for you to
send." (in the
row's "If you approve:" line), "Applying cancels …" (Manage bundles),
"Adopting it cancels …" (Drift's Adopt), "Revoking it cancels …" (Drift's
Revoke), and on Zitadel › Users, where each role is its own planned act and
the line is every role's plans together: "Revoking it cancels …" / "Revoking
them cancels …", "Giving them cancels …", "Replacing them cancels …" (task
1.9). A Zitadel › Users write whose plans moved writes nothing: "Nothing was
changed. Their access moved while this was open." with "Read the lines above
again, then press Give roles in Zitadel again." (the button's own words). A
revoke there of a role Syndra still gives is kept: "Keeps Member in Laser
Lab, because a bundle, a direct grant or an automatic rule still gives it."
Extend on a row (Expiring access,
Home, a person's page) reads its plan at the press, so it pushes the step
then ("Cancel it and extend"). The press
pushes the cancel step at once; an act that cancels nothing gains no step.
While the plan is read the act is held: "Checking what giving it does." /
"Checking what approving does."; unread: "Held until Syndra can read what
approving does." and "Syndra couldn't read what approving does, so it can't
show what else this would give or cancel. Try again to read it." What the
automatic rules add is the planner's sentence, one source: "Also gives
Trained in Laser Lab, by the automatic rule from Member in Laser Lab."

**What set a change off, and why** (architecture deepening 1.6a, 1.8a). A
rule's queued change names the role one step before it in a chain, as it was
when the change was queued, so editing the rule later renames nothing: "Rule
from Trained in Laser Lab" on Pending changes, "by the automatic rule from
Trained in Laser Lab" in Change history (on a rule's write inside a bundle's
change, or one step further down than the rule edited). A rule since retired,
or unknown: "Automatic rule" on Pending changes, "by an automatic rule" in
Change history. The operator's reason is quoted, “term start”, once on each
line that shows the change: under the event on the Audit page, a person's
Activity and Home (a bundle given or taken, a role given, extended or
removed, a sign-in lookup), and under each Zitadel row on Pending changes.
Nothing when none was given, and Syndra's own words are never quoted as a
reason: the reason column also holds Syndra's labels ("Mapping rule
cascade", "Extended from Home", "Direct access removal" …), so a surface
quotes only the backend's `said` (`db.Said`, one list, guarded in
`repoguard/said_labels_test.go`), never `reason`. A new fixed reason
goes on that list in the same change. Where
it goes is said truly: the bulk dialog's "Syndra keeps it with each person's
change." and the sign-in lookup's "It's on the Audit page with your reason."
(it said Change history, which holds bundle and rule changes only).

**A revoke that only cancels is the cancel, said plainly** (owner, 8 Nov: "give" is only a verb, so the sheet says what was going to happen, to whom, and that it has not). Lede: "Given to
Liv on 3 Nov by Randy Orton, and still waiting for you to send." Title: "Cancel giving Member in
Laser Lab to Liv?" (never "Revoke direct access to …", which is not the act). Body: "It's still waiting for you to send, so Liv never had it. Nothing is sent to Zitadel."
Button: "Cancel giving it" ("Cancel giving the Safety bundle to Liv?" for a bundle, same buttons;
"Cancel removing Member in Laser Lab from Randy?" and "Cancel removing it" for a waiting removal); the
dismiss beside it is "Keep it waiting", never a second "Cancel". Once pressed, the sheet
is its result (A-22 G22): the lede, body and before-line leave with the form. Result: **Cancelled**, "Member in
Laser Lab won't be given to Liv. It never reached Zitadel." / "Member in Laser Lab stays with Randy. The removal never reached Zitadel.", then "Nothing for you
to do."

**A plan is grouped by how each change travels** (A-21 §Q1, architecture
deepening 1.7, `lib/plan-groups.ts`). People, not writes; one row per person;
the heading says which, the row says what, in the planner's words, before and
after Apply. Folds, in this order, and a fold with nobody in it is not drawn:

| Before Apply | After Apply | Dot | Faint line under it |
|---|---|---|---|
| Straight away | Done | none; after, healthy, blooms once when every written row reached | none |
| Will wait for you to send | Waiting for you to send → (the link) | amber | "Because Trained in Laser Lab always waits for you." / "Because A and B always wait for you." |
| (after only) | Zitadel didn't answer | amber, still | the retry line ("Zitadel didn't answer. Syndra sends them again at 14:05 (try 2 of 6).") |
| (after only) | Zitadel refused | danger, still | none; each row: "Zitadel refused this: {cause}. {step}" |
| Left as they are | Left as they were | none | none; each row keeps its Badge, No change or Refused |

Counts: "1 person" / "4 people" (the dialog's own noun). The footer's
before-line: "Every change goes to Zitadel when you apply." · "Every change
waits for you to send. Nothing reaches Zitadel until you press Send on Pending
changes." · "Apply sends 6 changes to Zitadel now. 3 wait for you to send."
The before-line and the result clauses count changes (each row's roles that
go and wait), never people: a person under the wait fold for one role still
has the rest sent straight away (browser pass, 8 Nov; it said "7 people's
changes"). The fold headings and the result word still count people. · then, with a row that only cancels, "For 1, this only
cancels a change that's waiting; nothing is sent." / "For 2, this only cancels
changes that are waiting; nothing is sent." · nothing to apply: "Nothing here
would change, so there is nothing to apply." (said once, as the disabled
button's reason). The result: the word ("Given to 11 people", "Given to 9 of
11 people", "Revoked from …", "Extended for …"), then one clause per fold in
fold order, in changes: "All 11 are in Zitadel now." / "6 are in Zitadel now." · "3 wait
for you to send." / "All 11 wait for you to send. Nothing has reached Zitadel
yet." · "1 isn't in Zitadel yet; Syndra sends it again on its own." / "2
aren't in Zitadel yet; Syndra sends them again on its own." · "Zitadel refused
1; the row says why." / "Zitadel refused 2; each row says why."; then one
step: the Send link, "Follow the step on each row Zitadel refused, below.",
the retry's "Nothing for you to do. Syndra sends them again at 14:05 …", or
"Nothing for you to do." Retired: the "Will change" badge on a grouped plan
(flat plans keep it), "No change for N · N refused. Syndra leaves those as they
are.", "Every selected person will change.", "N more changes from automatic
rules wait in Pending changes for you to send." (a rule's role counts in the
person's fold), and the sentences that overwrote a row after Apply ("Recorded
in Syndra. A role it gives always waits for you, …").

**When a person's access moved while the sheet was open, nothing is written**
(A-21 §3.3). In a sheet, one Notice: "Nothing was changed. Liv's access
moved while this was open." with "Read the note under the form, then press
Give role again." (the button's own label), and the note is the fresh plan. In
a row (Approve): "Nothing was changed. Liv's access moved while you looked."
then "Read the line above, then approve again." Extend on a row: "Nothing was
changed. Liv's access moved while you looked." then "Press Extend again to
read it afresh." Drift's Adopt sheet: "Nothing was changed. Liv's access
moved while this was open." with "Read the lines under the role, then press
Adopt in Syndra again." Never "stale" or "plan".

**Extending sends nothing to Zitadel** (architecture deepening 1.2a): Zitadel
has no end dates, so an extension is never "queued", never "waits for you" and
never "once sent". The result is **Extended**, "Extended by 90 days", then the
plan's own sentence ("Keeps Member in Laser Lab until 3 Dec 2026.") and the
row's closing line; then "Nothing for you to do.". A plan that refuses says
the planner's words ("Already ends later than this would make it. Nothing was
changed.") and its step.

**Adopting says the planner's sentences** (1.3): what it records ("Records a
direct grant, so the access stops being unexplained and starts being owned.
Nothing is sent to Zitadel for it."), then each "Also gives …" and "Giving …
waits for you to send.". What the rules give is sent straight away, so the
result says "1 more change from automatic rules waits in Pending changes for
you to send." only when some of it waits.

**Manage bundles says whether it finishes now or waits**, from the plans:
"This goes to Zitadel when you apply." / "This waits for you to send." /
"Apply sends 2 changes to Zitadel now. 1 waits for you to send." / "For 1,
this only cancels a change that's waiting; nothing is sent." Every staged
change's cancels are one step: "Cancel all 3 and apply 2 changes".

**Extending says nothing goes to Zitadel, because nothing does** (architecture
deepening 1.2a; settled 5 Nov: Zitadel has no end dates, Syndra ends access on
the day). The plan: "Keeps Member in Laser Lab until 3 Dec 2026." then
"Nothing is sent to Zitadel, which has no end dates. Syndra ends the access on
the day." An extension never waits for you and is never "queued"; its forecast
is the act's own ("Will be extended"). It asks before it cancels a removal
still waiting: the press reads the plan and pushes the step ("Cancel it and
extend"). Refused: "Already ends later than this would make it.", "Has
no direct grant with an end to extend." or, ticked rows that have gone, "None
of the selected grants are theirs any more." with "Read the list again to see
what they have now."

**Every bulk row is the planner's sentence** (architecture deepening 1.3),
the same words the single act says: a give "Gains Member in Laser Lab." then
"Expires 3 Dec 2026." / "No expiry."; a removal "Loses …" or, with no direct
grant, "Holds no direct grant here." with "Keeps the role another way, through
a bundle or an automatic rule. Remove that instead."; an approval the single
approval's words, a refusal ending "Decline it to close the request."; an
adoption "Adopted into Syndra (Member in Laser Lab)." then "Records a direct
grant, so the access stops being unexplained and starts being owned. Nothing
is sent to Zitadel for it."; a publish or move "Moves from v2 to v3." then
"Gains …" / "Loses …", "Stays on v2.", "Already on v3.", or "Everything in
this version they already hold another way, so nothing is sent to Zitadel."
What the rules add, what waits ("Giving Trained in Laser Lab waits for you to
send.") and what it cancels follow, in that order. Role names, never keys; no
arrows.

**The drain switch is said only once it is read.** Until the indicators answer (or when they can't), nothing says "on its own" or "until you press Send": the sentence says what is known and ends on "Open Pending changes to see what has not reached Zitadel yet." The rail pill says "Syndra stopped sending N changes", the same words as Home and Pending changes.

## 6. Vocabulary

One word for one thing, everywhere, and the precise word in preference to a
paraphrase of it. Where the precise word is a term of art, it carries its
definition with it rather than being replaced by a plainer, vaguer one.

### The mechanism

`lib/glossary.ts` holds every defined term, once. `<Term name="cascade">` sets
the word in the sentence and puts the definition one hover, tap or Tab away.

Mark up a term's **first appearance on each page**, and only the first. A page
that marks up every occurrence is a page of dotted underlines; a page that
marks up none has left a reader with nowhere to go. A member who opens
*Network storage* has not read *Bundles*, so "first use" is per page, not per
product.

Three kinds live in the glossary, and the difference decides how the copy
around them reads:

- **Products** — Zitadel, TrueNAS, Google Workspace. Use the bare name. Never
  gloss inline: no "Zitadel (the service everyone signs in through)". Marked
  up on first use so a new colleague in their first week has somewhere to look.
- **Standard terms** — grant, entitlement, provision, reconciliation, drift,
  claim, token, OIDC, SAML, role key, service account. The field's own
  vocabulary. Use it plainly. The definition is a courtesy, not a lesson.
- **Syndra's own** — bundle, automatic rule, cascade, outbox, drain, hold,
  mapping, merge finding, adopt, baseline, log anchor, unvouched, intent
  ledger. Nobody knows these from experience, however senior. Their
  definitions carry the consequence, not just the meaning, because they are
  the only explanation the product ever offers.

Adding a term to the glossary is how a word becomes permitted. A term of art
that is *not* in the glossary is not licensed by this section — it is
unexplained jargon, and the fix is to define it, not to paraphrase it away.

### One name per thing

These are not difficulty problems, they are consistency problems: the same
thing under two names reads as two things.

| Say | Not |
|---|---|
| **Zitadel** | the identity provider (as a name), the provider, the directory, the catalogue, upstream, downstream, IdP |
| **TrueNAS**, or the system's own name via `targetLabel` | the target, the NAS |
| **connected system** (the class) | target, integration |
| **person / people** | user, subject, principal, holder (as a noun) |
| **makerspace staff** — in member-facing copy | lab manager, steward, operator, whoever runs the makerspace |
| **revoke** — end a person's access | withdraw access, take away, drop, retire access |
| **withdraw** — a member takes back their own request | — |
| **cancel** — undo a change still waiting for you to send ("Cancel giving it", "Cancel removing it", the row verb "Cancel"; the dismiss is "Keep it waiting"; result **Cancelled**). A waiting change is named by what it was going to do ("giving Member in Laser Lab to Randy", "removing it from Randy"), never "the waiting give" | withdraw the queued grant, withdraw the queued change, withdraw (of a queued change) (A-21 §3.2, 8 Nov) |
| **remove** — take a thing out of a set | — |
| **delete** — an object ceases to exist | purge |
| **retire** — a bundle closes to new members | delete (of a bundle) |
| **preview** then **apply** | rehearse, dry run, simulate |
| **send** — dispatch what is waiting | flush |
| **decline** | deny, reject |
| **lift** — end a hold | release, unblock |
| **give** — a role, a bundle (result: **Given**; audit: "Gave a role"). **A verb only (owner, 8 Nov 2026):** never "the give", "a waiting give", "give-up", "gives of"; where a sentence needs a noun it names what is waiting, the role and the person, or says "stopped sending". Guarded by `plain-language.test.ts`. | grant direct access, grant access, granted (owner, A-17 Q7, 5 Nov). "Direct access" stays only as the noun for what *revoke* ends; the stored `direct_grant.*` codes and "Grant id" in lineage are technical and unchanged |

*Operator* names an audience and a view. It is correct on staff screens and
wrong in anything a member reads.

### States

One set of words for a change's life, everywhere it is reported:

**waiting to be sent** → **sent** → (**failed**, and Syndra will retry) →
(**given up**, and a person must act). Plus **no change** (already in that
state) and **refused** (Syndra declined, and says why).

### Pages

| Nav label | What its lede says it is |
|---|---|
| **Home** | what needs you today |
| **People** | everyone Syndra knows, and what each can use |
| **Projects / Roles / Apps** | what access is made of |
| **Requests** | what members have asked for |
| **Bundles** | sets of roles handed out together |
| **Automatic rules** | roles that follow from other roles |
| **Pending changes** | changes waiting for you to send |
| **Change history** | what each edit set off, and whether it landed |
| **Access map** | how roles, bundles and rules connect |
| **Drift** | access with no record explaining it |
| **Unfinished revocations** | access somebody revoked that is still there |
| **Expiring access** | access that ends soon, whether or not you look |
| **Holds due** | holds past their review date, still in force |
| **Audit** | what people did in Syndra |
| **Zitadel** | whether Syndra can reach it, and what it holds |
| **Connected systems** | the systems Syndra maintains accounts on |
| **Incoming events** | what reached Syndra from outside |

A page's title names **everything** the page shows, never one of its parts.
If a filter offers a source the title does not admit, the title is wrong —
that is how *Incoming events* spent a release called *Zitadel events* while
carrying onboarding triggers Zitadel never sent.

### An app's claims page (A-15, A-16)

| Say | For |
|---|---|
| **The roles each person holds in {project}, as a list** | the roles claim's line ("as text" when sent as text) |
| **All 7 roles, in Syndra's words.** · **Both roles, …** · **Its one role, …** · **All 7 roles, in NetBird's words.** · **6 of 7 roles. Staff as `netbird-staff`; Guests left out.** · **5 of 7 roles. 2 renamed, 1 left out.** | the roles line's faint state (`rolesState`) |
| **New in this draft.** | a line the saved shape lacks |
| **No roles. {app} gets none from Syndra.** | the first line when nothing carries roles |
| **Zitadel sends `email`** · **Sent as `groups`** · **Then choose the roles** · **Then name it and set it** | the add menu's right seat: why an entry is muted, or the one choice still to make (`ADD_MENU`) |
| **Add a claim inside** · **Nothing: top level** · **A new object** | an object's editor, and the Inside field |
| **Name it and give it a value.** · **Give it a value.** | a fixed value that isn't finished |
| ~~Claim rows~~ | gone with its fold (A-16) |
| **Show the checks** / **Hide the checks** | the preview's trail, one step away while everything passes |
| **All five checks pass** · **Check 1 fails** · **Checks 1 and 5 fail** · **Check 4 needs a look** | the verdict line, each with " on this draft" when a draft is previewed (`verdictOf`) |

### Still never on screen

Not jargon — noise, or the developer talking:

`payload`, `mutation`, `idempotent`, `fixture`, `seeder`, `hydrate`,
`truncated` (say *cut short*), `degraded` (say what could not be read),
`manifest`, `node`, `edge`, `hop`, and any raw identifier where a name
resolves. No `please`, no `sorry`, no `oops`, no exclamation marks.

Design rationale never appears on screen. A sentence explaining why a section
keeps its seat is written for whoever maintains the code, and belongs in the
code.

## 7. Members and staff read differently

Same facts, same vocabulary, different register. A member is not an
administrator, so their copy names a person to ask rather than a mechanism to
understand — but it does not hide which system they are dealing with. A
student whose share will not mount searches for *TrueNAS*, not for "the
network storage server".

| Fact | To staff | To a member |
|---|---|---|
| a role held via a bundle | "Via Lab Tech (bundle)" | "Because you're in Lab Tech" |
| access expiring | "Expires 12 Sep" | "Ends 12 Sep — ask before then if you still need it" |
| a hold | "On hold until 3 Oct: safety refresher" | "Paused. Makerspace staff will look again on 3 Oct. If you think this is a mistake, ask them and mention the reason shown here." |
| a queued change | "Waiting to be sent to TrueNAS" | "Your TrueNAS access is on its way. It usually takes a few minutes." |
| the sign-in service | "Zitadel" | "Zitadel" — marked up, so the definition is one tap away |

Members never meet *operator*, *drift*, *cascade*, *outbox* or *reconcile*:
those name work they do not do. They do meet the products they use, by name.

## 8. Errors and system failures

- Say which thing failed and which did not: "TrueNAS did not answer. Syndra
  itself is fine, and nothing was changed."
- Say the state of the world: "Nothing was changed." "Your request was
  saved; the confirmation did not send."
- Say the next step, and where: "Try again in a minute. If it keeps failing,
  ask the person who runs the Syndra server and quote the reference below."
- The reference is called a **reference**, is shown in full, and carries the
  line "Quote this if you ask for help."
- Never show a stack, a code path, an HTTP status, or a raw identifier as the
  message. Those may sit under a "Technical detail" disclosure for the person
  who runs the server.

## 9. Accessibility copy

- A button's visible text is its accessible name. `aria-label` is used only
  for an icon-only control, and then says the whole action: "Copy your
  account name", not "Copy".
- Colour never carries a meaning alone. Every dot, tint and badge has a word
  beside it or a visually-hidden one inside it.
- Status regions (`role="status"`, `role="alert"`) always contain a sentence.
- A field's label is its name; its hint is a separate element (`FieldHint`),
  never inside the label. Errors are associated with the field they concern
  and say what to change.
- Links say where they go: "Open Pending changes", never "here" or "→" alone.
- Counts in links say what they count: "12 people", never "12 →".
- Placeholders never carry instructions. Instructions go in the hint.

## 10. What the guard checks

`plain-language.test.ts` reads every component's copy — JSX text and string
literals, through a tokenizer that knows where code and comments are, and that
reads a capitalised one-word literal as a label — and fails on:

1. a name from the *one name per thing* table's right-hand column;
2. a word from *still never on screen*;
3. a `PageHeader` with no `lede`;
4. a sentence in `meta`;
5. an exclamation mark, or `please` / `sorry` / `oops`;
6. a bare `Dismiss`, `OK`, `Submit`, `Confirm`, `Go` as a whole button label;
7. an `aria-label` that is a single verb;
8. a glossary term used on a page that never marks up any term — the
   definition is reachable from somewhere on that page, or the term is
   unexplained jargon again;
9. an `ARGUED` entry whose file no longer exists.

Exceptions are argued in `ARGUED`, one reason each, and that map is the record
of every place the product chose to break its own rule.

What it cannot check is whether a sentence is **true**. Every word in "Zitadel
events" was permitted vocabulary; the claim was false. A title that names part
of its page, a timing that does not match the scheduler, a sentence true in
one branch and false in another — those are found by reading the copy against
the code that produces it, and nothing here substitutes for that.

## 11. Messages in their place (A-22, owner 8 Nov 2026: yes to Q1 to Q7)

Rules G20 to G31 (`design_handoff_syndra/reconcile-2026-10/RULES.md`, "Messages (A-22)"); the sweep is `from-design/A-22-every-message-in-its-place.md`.

- **Title, dismiss, act.** The title is the act in the button's words, a question mark only when the act takes something away (Q2); the act names its verb and object; the dismiss says what it keeps: **Keep it**, **Keep them**, **Keep them open**, **Keep it waiting** (a cancel of a waiting change), **Keep tracking it**, for acts that take away, retire, cancel or decline; **Cancel** only on forms that create, ask, give or record; **Back** a step behind; **Close** after a result (Q1). Two buttons in a pair never share a verb.
- **After a result the sheet is its result** (Q3): the lede, the What-happens list, the tick and the before-line leave with the form; the header keeps chip and title; the footer is one outline Close. A failed press keeps the form so it can be pressed again.
- **One result slot per surface**, cleared by the press before it writes (G23).
- **Words.** Revoke direct access and Drift: forecast **Will be revoked**, **Revoked**, **Not revoked**; **Removed** stays for a bundle out of a person's set (Q4). Manage bundles: **Will be applied**, **Recorded**, **Not applied**. Every failure is "Not" plus the act's word; "Refused by Zitadel" when Syndra recorded it and Zitadel refused. Retired for good (the guard fails on them): Didn't go through, Couldn't put it on hold, Couldn't lift it, That didn't save., That did not go through., Revocation recorded, Not all sent.
- **Send** (Pending changes, Home): **Sent 7 of 9** (changes counted, the whole said), **Sent 9**, **Not sent** (none went, or another send was running), **Nothing to send**. When the queue is empty after the send the empty state is the result and no card is drawn (Q6).
- **Colour follows what happens to the person** (G25): red when a named person loses access that reaches a system, amber when a step is left or a state must be read, quiet for what only records, asks, previews or cancels something unsent.

## 12. What an add-on says about a person (addon-contract slice 6a)

One set of generic renderers (`ui/src/components/results/`) draws every member-scope read an add-on answers, by its shape (records, status, activity) and never by what the add-on is. `components/results/copy.ts` holds the sentences; `resultsCopyGuide.test.ts` fails when this table and that file disagree. Every outcome ends on a next step or "Nothing for you to do."; `{name}` is the system's name as the page calls it.

| When | Says |
|---|---|
| A list with nothing in it, to a member | {name} lists nothing for you. Nothing for you to do. |
| A list with nothing in it, to an operator | {name} lists nothing for {person}. Nothing for you to do. |
| A status with no meters, nothing unread | {name} reports no usage. Nothing for you to do. |
| An activity log with no events | No recorded activity on {name}. Nothing for you to do. |
| A read that did not come back: title | One read from {name} did not come back |
| A read that did not come back: body | Syndra asked {name} and did not get a usable answer. This is not a claim that nothing is there. |
| A read that did not come back: next step, to a member | Try again in a minute. If it keeps failing, ask makerspace staff. |
| A read that did not come back: next step, to an operator | Try again in a minute. If it keeps failing, check the add-on's health on its system page. |
| Usage the add-on could not read, some of it | Some usage could not be read from {name}. |
| Usage the add-on could not read, all of it | Usage could not be read from {name}. |
| Usage the add-on could not read: body, some of it | The usage above is what was read. |
| Usage the add-on could not read: body, all of it | This is not a claim that nothing is used. |
| A meter with no ceiling | no limit |
| A meter's ceiling | of {limit} |
| A record held since a date | Since {date} |
| A record due on a date | Due {date} |
| A records list's column heading, the record's name (sorts the list) | Record |
| A records list's column heading, held since (sorts the list) | Since |
| A records list's column heading, due (sorts the list; the list opens on it) | Due |
| What a screen reader hears after "Sort by", the record's name | name |
| The same, held since | date held |
| The same, due | due date |
| What the add-on adds that Syndra does not know (heading) | What the add-on also says |
| More of it than is shown | …and {n} more |
| One place that was not recording, to a member | {place} did not record you, so nothing from that place appears here, whether or not it happened. Nothing for you to do. |
| Several places that were not recording, to a member | {place} and {place} did not record you, so nothing from those places appears here, whether or not it happened. Nothing for you to do. |
| One place that was not recording, to an operator | {place} did not record {person}, so nothing from that place appears here, whether or not it happened. Switch recording on there. |
| Several places that were not recording, to an operator | {place} and {place} did not record {person}, so nothing from those places appears here, whether or not it happened. Switch recording on there. |
| Under an empty activity list when some places were not recording | Some places were not recording this account, so this is not the same as nothing having happened. See below. |

A record's lateness is the add-on's word (its standing note and next step, for example "Overdue since 3 November" and "Bring it back to the front desk"), never worked out in the UI from a date. The meters that were read stay when some usage was not; empty meters never stand in for "nothing used". Records open in their most logical order, soonest due first (so overdue comes before due soon), then records with no due date, then by name (owner, 13 Nov 2026), and the Record, Since and Due headings re-sort the list (`SortHead`; a record with no date in the column in force always sits last). Meters and uncovered places stay alphabetical (case aside), since a place's name is what a person looks up. Where a sentence differs by reader it is one row per reader, and the code takes the reader (`Reader` in `copy.ts`) rather than choosing words inline. The storage kind says "shares" for the same places in `kinds/storage/ActivityCoverage.tsx`.

## 13. How fresh a read is (addon-contract slice 6b)

Every read of a system carries when it was read, and an add-on may answer from its own copy (feature `freshness`). The sentence and the one control that is its next step come from `components/results/copy.ts` (the read line) and `lib/outcome.ts` (a refusal); `freshnessCopyGuide.test.ts` fails when this table and those files disagree. The words on the control are the product's: **Read again**, and **Reading** while it runs. `{name}` is the system's name as the page calls it, `{age}` is "4 min ago", `{n}` whole seconds, `{time}` a clock time. A page load and a poll never ask the system itself; only the control does.

| When | Says |
|---|---|
| A copy the add-on kept (the control is the next step) | {name} was read {age} |
| Read again, or a page load, refused inside the cooldown, with a copy or without (the control waits, then the line above; this is the whole message, there is no refusal block and no "nothing was changed", because nothing was read); and a "Check now" pass skipped inside the cooldown that is not going again by itself (a second skip, or **Stop waiting**), followed by its step | {name} was read moments ago; you can read again in {n} s |
| That "Check now" step, while the count runs | Press Check now again when the count ends. |
| That "Check now" step, once the count is over | Press Check now again. |
| Read again failed for any reason but the cooldown (the line stays as it was; nothing was read, so the failure is said beside the press, in the outcome every refusal wears: under the line on a single-card page, above the cards where one press refreshes several, as on the storage, member-system and person-systems pages) | **Not read**, then the failure's own sentence and step (`failureMessage`, never a status code) |
| The system cannot be reached, to a member (no control) | {name} can't be reached right now; this is what it said at {time}. Try again in a minute. If it keeps failing, ask makerspace staff. |
| The system cannot be reached, to an operator (no control) | {name} can't be reached right now; this is what it said at {time}. Try again in a minute. If it keeps failing, check the add-on's health on its system page. |
| "Check now" or a removal refused inside the cooldown, waiting to go again by itself, once: the reason under the control, which is off until then (the **Stop waiting** control beside it stops it, and the refusal is then shown as the rows around this one say; so is a second refusal) | {name} was read moments ago; trying again by itself in {n} s |
| A removal refused inside the cooldown (a write; the one place a refusal block is shown): the refusal | That system was read moments ago, so nothing was changed |
| The same refusal: its next step, counting down | You can try again in {n} s. |
| The same refusal: its next step once the count is over | Press the same button once more. |

A 429 is a cooldown only when it carries the code `FRESH_READ_COOLDOWN` (a proxy's own 429 is an ordinary failure). The count starts from the add-on's `retry_after` (or the Retry-After header), from when the answer arrived, and is a deadline, never a tally of ticks: leaving the page and coming back does not restart it, and a second refusal does. A copy the add-on kept is drawn neutral (grey dot, faint text) at any age, because it is the normal state. A cooldown is amber when the reader caused it, and a cooldown carried in the data always is (only a press reads fresh); a page load inside somebody else's wait is marked as such at the load site and is neutral, like a copy. The colour is chosen by what the line says: the cooldown while it counts, then a copy is neutral when it is not current and anything else is drawn by its age. An unreachable system stays amber. Every cooldown counts from when its answer arrived (the query's `dataUpdatedAt`, or the time of the refusal), wherever it is drawn. The control is off, with the line beside it as its reason, until the count ends. "Check now" and a removal refused inside the cooldown wait the count out and go again by themselves, once (RFC 9110 Retry-After), never longer than a minute, with **Stop waiting** beside the line; a second refusal is shown as the rows below say. A read that is only a copy keeps every binding action blocked (Adopt stays off until Read again returns a current read); the unreachable line has no control because only time helps. The words are never "Refresh".

## 14. Acting on things in the world (addon-contract slice 6c)

A member can do something to a thing on a system that an add-on declares `physical` (today, `item.checkout`): press the action, see what will happen, press **Confirm**. The page knows nothing about items; it draws the add-on's declared actions and parameters. `components/actions/copy.ts` holds every sentence; `actionsCopyGuide.test.ts` fails when this table and that file disagree, and the lint guard refuses an inline sentence in `components/actions/*.tsx` (as in section 12). `{param}` is the parameter's name in words ("item id"), `{n}`, `{limit}` and `{unit}` come from the refusal's details; the value a member typed is never in a sentence or a message the server sends. Every outcome ends on a next step or "Nothing for you to do."

**Nothing here may suggest the member caused what is not theirs.** A system that does not answer, a record still being settled, an item someone else has: each says what is true about the system or the item, never "you failed", "you forgot" or "you chose wrong". The words are **Done**, **Nothing changed**, **Not done**, **Checking** and **Not sure yet**; a refusal's word is always **Not done**, and the page words it from the error code (`refusal` in `copy.ts`), never from the server's message.

| When | Says |
|---|---|
| The form | Say which one. Nothing is sent until you confirm. |
| The id field's hint | Type the id on its label, or scan it. |
| The camera button (only where the browser can read labels) | Scan with camera |
| The camera dialog: title | Scan the label |
| The camera dialog: body | Hold the label in front of the camera. The id is filled in when it reads it. |
| The camera is starting | Starting the camera |
| The camera cannot be used | The camera could not be used. Type the id instead. |
| The preview's button | Show what will happen |
| The preview: lede | This is what will happen. Nothing has been sent yet. |
| The preview: under the buttons | Nothing is sent to {name} until you press Confirm. |
| The preview: they already have it (no Confirm) | You already have this, so there is nothing to confirm. Nothing for you to do. |
| The preview could not be read: title | The preview could not be shown |
| The preview could not be read: body | {name} answered, but Syndra could not read what it said. Nothing was sent. |
| The preview could not be read: next step | Go back and try again. |
| The confirm's button | Confirm |
| The action while their last one is still being settled (the button is off, this is its reason) | Your last one is still being checked. Wait for it before trying again. |
| The same, once it is still in doubt after an hour | We still can't tell whether your last one went through. Ask at the desk. |
| It went through | It went through |
| It was already theirs | You already have this |
| The system did not do it | It did not happen. Nothing was changed |
| The system did not do it: next step | You can try again. |
| The answer was lost: the sentence | We're checking whether it went through |
| The answer was lost: the detail | It was sent, and the answer did not come back. |
| The answer was lost: next step | Nothing for you to do. This page checks again on its own, and Syndra will not send it again. |
| The answer was lost and the last ask did not get through | We could not check just now. This page tries again on its own. |
| The answer was lost and no record of it can be found: the sentence | We can't tell yet whether it went through |
| The answer was lost and no record of it can be found: the detail | It was sent, and the answer did not come back. |
| The answer was lost and no record of it can be found: next step | Reload this page in a minute to see where it stands. Do not try again before then. |
| Still in doubt after an hour: the sentence | We still can't tell whether it went through |
| Still in doubt after an hour: the detail | Staff have been told. |
| Still in doubt after an hour: next step | Ask at the desk. |
| Unknown id | Nothing matches that {param}. Nothing was changed. Check the label and type it again. |
| Their roles do not allow it | Your roles do not allow this for that {param}. Nothing was changed. Ask makerspace staff, or ask for access in Requests. |
| Someone else has it | Someone else has that one. Nothing was changed. Pick another, or ask at the desk. |
| At their limit | You are at your limit: {n} of {limit} {unit}. Nothing was changed. Return one first, then try again. |
| Their last one is still open | Your last one is still being checked. Nothing was changed. Wait for it before trying again. |
| Refused for a reason it did not give | That system refused this and gave no reason. Nothing was changed. Ask makerspace staff. |
| No role reaches the system | None of your roles reaches this system. Nothing was changed. Ask for access in Requests. |
| Their account is not made | Your account here is not made yet. Nothing was changed. Nothing for you to do. Syndra makes it. |
| The system is paused | Changes on that system are paused. Nothing was changed. Try again later. If it stays paused, ask makerspace staff. |
| The system cannot do it now | That system cannot do this right now. Nothing was changed. Try again in a minute. If it keeps failing, ask makerspace staff. |
| The system does not offer it | This system does not offer that. Nothing was changed. Reload the page to see what it offers now. |
| The system is not answering | That system is not answering. Nothing went out. Try again in a minute. If it keeps failing, ask makerspace staff. |
| The preview got no answer | That system did not answer the preview. Nothing went out. Try again in a minute. If it keeps failing, ask makerspace staff. |
| Syndra could not check their last one | We could not check on your last one. Nothing went out. Try again in a minute. |
| Too many tries | That was tried too many times in a short while. Nothing was changed. Wait a few minutes, then try again. |
| Too many requests (the shared per-member limit) | Syndra is getting too many requests from you right now. Nothing was changed. Wait a minute, then try again. |

The dialog is one dialog: the form, the preview, the camera and the result are its steps, never a second sheet on top. A confirm is sent once. When its answer is lost the page asks `GET .../operations/{id}` every 5 s (the server limits a member to 60 a minute) until it settles; it never sends again, and the action stays off while any confirm of theirs is open, on a reload too. A refusal that leaves something to fix (unknown id, taken, a limit) keeps a **Back** to the form.

**On the operator's list (Waiting on you).** A checkout whose answer was lost, once it has been in doubt for an hour, is a row like every other unconfirmed change: it says what was asked, **Ask at the desk whether {item} went out to {person}. If it did, it happened.**, and takes the same two answers. `checkoutWording` in `copy.ts` holds those words.

| Row | Says |
|---|---|
| What was asked | Check out {item} to {person} |
| The question | Did {item} go out to {person}? |
| How to tell | Ask at the desk whether {item} went out to {person}. If it did, it happened. |
| It happened | {item} went out to {person}. |
| It didn't happen | {item} did not go out to {person}. |
| After "It happened" | {item} went out to {person} on {name}. |
| After "It didn't happen" | You recorded that {item} did not go out to {person}. |

**Assigning an account, and letting one go, on the same list.** An assign (`account.adopt`) or a stop-tracking (`account.release`) whose answer was lost waits here too, unless the system's own record settles it (addon-contract 6.3r). Syndra's lists of accounts read Syndra's copy of the binding, which changes only when the system confirms, and a person's answer records the answer alone. So How to tell never points at those lists: the next step is to do it again, which the system answers when it already happened and refuses for an account someone else now holds. Every answer ends with a next step. `wording` in `components/targets/UnconfirmedChanges.tsx` holds those words; `{account}` is the account's name, or "{person}'s account" when the record has none.

| Row | Assign (`account.adopt`) | Let go (`account.release`) |
|---|---|---|
| What was asked | Assign {account} to {person} | Let go of {person}'s account on {name}, keeping it |
| The question | Did {name} assign {account} to {person}? | Did {name} let go of {person}'s account? |
| How to tell | Syndra's lists change only when {name} confirms, so they can't tell you. Assign {account} to {person} again. If it already happened, {name} says so and Syndra lists it then. If someone else holds it now, {name} refuses. | Syndra's lists change only when {name} confirms, so they can't tell you. Stop tracking {account} again. If it already happened, {name} says so and Syndra stops listing it then. |
| It happened | {name} assigned {account} to {person}. Syndra lists it once you assign it again. | {name} let go of {account}. Syndra stops listing it once you stop tracking it again. |
| It didn't happen | {name} didn't assign {account}. | {name} still holds {account} for {person}. |
| After "It happened" | You recorded that {name} assigned {account} to {person}. Assign it again to have Syndra list it. | You recorded that {name} let go of {account}. Stop tracking it again to have Syndra stop listing it. |
| After "It didn't happen" | You recorded that {name} didn't assign {account} to {person}. Assign it again if {account} should be {person}'s. | You recorded that {name} didn't let go of {account}. Stop tracking it again if Syndra shouldn't manage it. |

## 15. What has been used (addon-contract slice 6e)

An add-on that reports statistics (feature `stats`) gets the same four surfaces with no code of its own: the member's card **What I have used**, the same card on a person's page, the target overview's totals, and Insights, Usage. `components/usage/copy.ts` holds every sentence; `usageCopyGuide.test.ts` fails when this table and that file disagree, and the lint guard refuses an inline sentence in `components/usage/*.tsx` (as in sections 12 and 14). Syndra's own sentences only: what the add-on wrote (a metric's label, a breakdown's name and value, a gap's reason) is escaped, capped (label 60, value 40, reason 120 characters) and, inside a sentence, put in quotes; this table never holds it. `{value}` is a number in its unit's words ("18.4 GiB", "3 items"), `{day}` a makerspace day ("9 Nov", with the year when it is not this year's; never "UTC"), `{range}` days ("8–9 Nov"), `{where}` a breakdown (`Area: “ring”`). Every card ends on a next step or "Nothing for you to do."

**A day nobody could count is not a zero.** It is hatched in the chart, the line breaks there, its hover says "couldn't count", and the card footnotes it with the add-on's reason. A counter's day with nothing in an observed window is a real 0 and is drawn as one; a day after the last one counted (`counted_through`) is not yet counted and is left blank, never 0. Each window is named from the day the backend read it from (`window_30d_from`, `window_12m_from`): the window's own first day, or the first day Syndra keeps for a person (`kept_from`, the operator's retention of N days counting today, so today − (N − 1)) when that is later. A window retention starts later is "in the last {n} days", n counted from its start to today, today included; a 12-month window it does not cut is "in the last 12 months"; when both start on one day the year line is not said twice. Only the drawn days before the 30-day start are hatched, with Syndra's own reason, and "Kept for only {n} days" is said only when such a day is drawn. A line joins only adjacent days that have a value: a gauge is never interpolated, and a value standing alone is a dot. **Ordered, never ranked** (the owner's ruling of 14 Nov 2026, superseding "Insights shows how many people and a spread, never who"). For operators, Insights shows each breakdown's spread and then **who**: every person with a value in the last 12 months, by the directory's name (never the id), linking to their page, in a table ordered by the last 30 days, largest first, whose headings re-sort (Person, Last 30 days, Last 12 months; a number's first press is largest first, as Operations' time is newest first). The spread has no floor: one person is still a spread. No sentence here, and no word in the renderers, says top, most, worst, best, highest, lowest, leader or rank, of people or of systems: the table is ordered, never labelled by place (`usageCopyGuide.test.ts` reads the files for those words). A member never sees another person's name, and their own hides nothing of theirs: on their own card a line whose breakdown the add-on wrote with someone else's name in it is left out and counted, and a gap that names someone else reads "The add-on's reason named a person." (the backend's sentence, quoted like any reason), on its own line. A series is never added to another.

| When | Says |
|---|---|
| The member's card: title | What I have used |
| The operator's card on a person's page: title | What {person} has used |
| A counter over its window | {value} in the last 30 days |
| The same, over the year | {value} in the last 12 months |
| The same, when retention starts either window later (n: its start to today, today included) | {value} in the last {n} days |
| A counted metric with no series | Nothing counted in the last 30 days |
| A member's own card: a line left out because the add-on named a person in it, one | 1 more line is not shown here, as the add-on named a person in it. |
| The same, several | {n} more lines are not shown here, as the add-on named a person in them. |
| A gauge's latest value | {value} on {day} |
| A gauge with no value in the window | No value in the last 30 days |
| A meter's ceiling (the limit Syndra observed) | of {limit} |
| A meter over a limit of 0 | no limit |
| A meter whose limit was not read | Its limit could not be read, so there is no meter. |
| Gaps: heading | Couldn't count these days |
| Gaps: one line, with its breakdown | {range}, {where}: {reason} |
| Gaps: one line, for the whole metric | {range}: {reason} |
| Gaps: a system that keeps per-person numbers for fewer days than the window | Kept for only {n} days on this system |
| Gaps: under the lines | A day left blank was not counted. It is not a zero. |
| A metric not collected yet | Not counted yet |
| A card with such a metric: its step | Nothing for you to do. The numbers appear here once Syndra has counted. |
| The add-on reports no statistics (target overview; a member or person page shows nothing) | This add-on does not report statistics. Nothing for you to do. |
| Syndra could not read its own numbers | Syndra couldn't read these numbers just now. Reload this page to try again. |
| Target overview: region title | What has been used here |
| Target overview: lede | Totals for everyone who used it, over the last 30 days and the last 12 months. One person's numbers are on their page. |
| When the last count ran | Last counted {age} |
| Accounts Syndra does not manage, one | {n} account Syndra does not manage also used it. It is in the totals. |
| Accounts Syndra does not manage, several | {n} accounts Syndra does not manage also used it. They are in the totals. |
| The retention control: title (the target overview, under the totals; the one place it is set) | How long per-person numbers are kept |
| The retention control: the field | Days |
| The retention control: its hint, which is the one statement of what is set | Per-person numbers are kept for {days} days. Totals are kept for 5 years. |
| The same, when an operator shortened it | Per-person numbers are kept for {days} days, as an operator set. Totals are kept for 5 years. |
| The retention control: save | Save |
| The retention control: back to the ceiling (only when a shorter value is set) | Keep the full 400 days |
| The retention control: nothing to save | Type a new number of days first. |
| Shortening asks first, in a dialog: its title | Keep per-person numbers for only {n} days? |
| Shortening asks first: the dialog's words | Per-person numbers older than {n} days are deleted within a day, and cannot be brought back. Totals stay for 5 years. |
| Shortening: the button that does it (the destructive fill) | Delete numbers older than {n} days |
| Shortening: the button that does not | Keep them |
| The retention control: the word on the result | Saved |
| The retention control: the result | Per-person numbers older than {n} days are deleted within a day. Totals stay for 5 years. |
| What the last count did not keep | The last count did not keep {reason}. |
| a value for a day that is not a whole day, one | {n} value that was not for a whole day |
| a value for a day that is not a whole day, several | {n} values that were not for a whole day |
| a value or gap that is empty or below zero, one | {n} value or gap that was empty or below zero |
| a value or gap that is empty or below zero, several | {n} values or gaps that were empty or below zero |
| a per-person value for a number that is not per person, one | {n} per-person value for a number that is not per person |
| a per-person value for a number that is not per person, several | {n} per-person values for a number that is not per person |
| a value for an account Syndra does not manage, one | {n} value for accounts Syndra does not manage |
| a value for an account Syndra does not manage, several | {n} values for accounts Syndra does not manage |
| a value with a breakdown the add-on did not declare, one | {n} value with a breakdown the add-on did not declare |
| a value with a breakdown the add-on did not declare, several | {n} values with a breakdown the add-on did not declare |
| a value older than Syndra keeps, one | {n} value older than Syndra keeps |
| a value older than Syndra keeps, several | {n} values older than Syndra keeps |
| a value from more series than Syndra keeps, one | {n} value from more series than Syndra keeps |
| a value from more series than Syndra keeps, several | {n} values from more series than Syndra keeps |
| Insights: tab | Usage |
| Insights: lede | How much of each thing is used across the makerspace: how it spreads over the people who used it, and what each of them used. Only operators see this. |
| Insights: the window | The last 30 days, from {day} |
| Insights: how many people, one | 1 person |
| Insights: how many people, several | {n} people |
| Insights: a system that keeps fewer days than the window | Over the last {n} days only, as that is how long this system keeps per-person numbers. |
| Insights: a system that keeps fewer days than the 12 months | The 12-month numbers are over the last {n} days only, as that is how long this system keeps per-person numbers. |
| Insights: a breakdown with people from the 12 months only (no spread, no bar) | Nobody used it in the last 30 days. |
| Insights: each breakdown's people, the table's name for a screen reader | {what}: each person's numbers |
| Insights: the people table's headings | Person |
| The same | Last 30 days |
| The same | Last 12 months |
| Insights: a gauge with no value in the last 30 days, in its cell | No value |
| Insights: what the id beside a person the directory could not name is (its copy control's name; the label is "Unknown account", never the id) | account id |
| Insights: the spread | Half are at {p50} or under; nine in ten are at {p90} or under. |
| Insights: nothing reports | No system reports per-person statistics yet. Nothing for you to do. |
| Insights: its step | Nothing for you to do. One person's numbers are on their page in People. |
| Insights: the link | Open People |
| Insights: loading | Loading the usage |
| Insights: could not read | Couldn't read the usage. |
| Insights: freshness subject | The usage |
| A chart's hover, a day with a value | {day}: {value} |
| A chart's hover, a day nobody could count | {day}: couldn't count |
| A chart's hover, a gauge's day with no new sample | {day}: no new value |
| A chart's hover, a day Syndra no longer keeps | {day}: kept for only {n} days on this system |
| A chart's hover, a day after the last one counted | {day}: not counted yet |
| A chart's description for a screen reader | {what}, day by day over the last 30 days |
| The same, with gaps | {what}, day by day over the last 30 days. Days that could not be counted are hatched |
| A unit's group heading, count | Counts |
| A unit's group heading, items | Items |
| A unit's group heading, bytes | Sizes |
| A unit's group heading, seconds | Durations |
| A unit's group heading, grams | Weights |

One meter draws every "used of limit" in the product (`components/ui/Meter.tsx`): 90% and over turns it warn and writes "nearly full" beside the bar, the limit itself writes "full", and the word is never left to the colour. Units are the registry's (`registry/units.json`): count plain, items "3 items", bytes in binary steps with one decimal ("18.4 GiB", as the storage card says it), seconds as minutes or hours, grams as g or kg. A meter is drawn over the limit Syndra **read** (the storage card's number), and when none was read it says so instead of guessing. Rows are in the order the backend sends them (alphabetical by label, then by breakdown).
