# Design reply: the add-on target plane

Everything I said in the course of this commission that is not drawn on a board. Written as
the design authority for the two boards in `design/`. Figure ids referenced here (`T1`, `S4`)
are the ids stamped into each figure's caption.

Boards:

- `design/Syndra Target Page.dc.html` — the redesigned target page. Figures T1–T5.
- `design/Syndra Target Screens.dc.html` — the six supporting screens. Figures S1–S15.
- `design/Syndra Mapping Screen.dc.html` — the mapping screen. Figures M1–M6.
- `design/Syndra Contradictions.dc.html` — the two states the code forced. Figures B1–B2.
- `design/Syndra Member Storage.dc.html` — the member's side of a pause. Figures C1–C3.
- `design/Sidebar.dc.html` — the existing rail component, included so the boards open
  standalone. Unchanged from the handoff bundle.

Commission 3 also changed four things on the commission-1 and commission-2 boards, listed in
section 4a. The boards in this zip are the corrected ones.

There is no build plan here, nothing is declared canonical, and nothing says to replace
anything. Where a drawing implies a component should exist, the component is drawn and
section 6 says what it does.

---

## 1 · What I decided about the target page's structure, and the argument for it

### One precondition, then three regions

The four-question spine survives as an argument and stops being four peers. Two of the
questions were never independent of each other, one of them grew into a second subject, and
the category the board did not anticipate — work waiting on a human — gets a seat with a name
rather than being distributed back into the four, which is how the current page happened.

**Question 1 and question 4 are one question.** A reachability reading has no meaning on its
own: *not answering* while somebody is draining it for a credential rotation is a different
fact from *not answering* at 04:00 on a Sunday, and an operator who reads the second when the
first is true walks to the wrong machine. Health and maintenance cannot sit six panels apart.
Together they are the page's precondition — *can anything I do here land, and whose machine is
it if not* — and they sit in a band above everything, beside the target's own readings,
because those are a second machine and never the same question.

**Question 3 absorbs the sweep.** What an add-on can do and what actually runs against it are
the same question asked twice; the manifest lists the operations and reconciliation is the
thing that calls them on a schedule.

The resulting order on the page:

1. **Is it answering** — the band. Syndra's reach on the left, what the target reports on the
   right, the maintenance state as the band's footer strip.
2. **Waiting on a person** — its own region, ranked second.
3. **People and their access here** — under a drawn seam and its own eyebrow. Three
   populations: Bound, Not created by Syndra, Lost their reason. Preceded by the mapping
   census line.
4. **What runs here** — the manifest beside reconciliation.

### Where the fifth category lives

**Its own region, ranked second.** Distributing it back into the four is exactly what produced
the page you have: two findings rendered inside Health, one panel two-thirds of the way down.
A difference reconciliation refuses to resolve is not a property of the target and not a
property of a person — it is a third kind of thing, and it is the only content on this page
that costs somebody access today.

It is second and not first because it is unreadable without the band: three findings on a
target that has not answered for forty minutes are three findings nobody can act on, and the
band is what says so.

### No tabs on the desktop

A tab bar satisfies *nothing appears with its count* and fails the thing that rule is for. The
four tab labels are identical on both pages; a finding under a tab nobody is looking at is a
finding nobody is looking at, and the only way to fix it is a badge on the tab — data driving
structure by another route.

**The desktop carries what the phone could not** because 1188px of content column puts four
region headings in one scroll, each one a sentence rather than a word, and the page difference
is carried by copy in a fixed place: the line under the title, and the region's own lede.
Nothing grows a panel; two sentences read differently.

### Two subjects, one page — and the seam is drawn

People and their access keeps its place on this page, under a rule and its own eyebrow, and
does not become a second screen. *Nothing bound* means one thing on a target that answered a
second ago and something else entirely on one that has not answered for forty minutes — split
them and the roster loses the only sentence that explains it. The seam is a hairline and a
heading, not a tab: an operator arriving with *why can this person not get in?* reads the band,
crosses the seam, and is on the roster in one scroll.

### What the mapping screen takes with it, and what stays

**It takes:** the mapping rows with their project, role key, field and value; the holder count;
Change and Remove; the published versions with their notes and rollback; and the
working-copy-versus-published distinction, which is the thing a table row cannot say —
"current version 4" meaning version 4 plus three unpublished edits is a sentence, and rolling
back from there undoes work listed nowhere. Per the build notes, the blast-radius
acknowledgement is a backend refusal producing a scope step, not a checkbox drawn upfront.

**The target page keeps:** two sentences and one outline control. How many mappings reach this
add-on and how many people hold those roles — the answer to *why is she bound here* — and a way
through to the screen that can change it. Navigation does not change: the mapping screen is
reached from this line and from the role, not from a new rail row, because a rail row per
add-on already exists and a second one for its mappings would be structure competing with
itself.

### The touch form (T5)

The touch board made this four horizontally scrolling tabs, and the desktop does not inherit
that. What the phone actually needs is a way to skip a region, not a way to hide three of them
— so the four names become an index at the top of one column, each carrying its count as a
number in the row rather than a badge on a tab, each a jump rather than a filter. Everything
below it is present and scrollable, so a finding cannot end up behind a tab nobody selected.

Three consequences of the same structure survive the narrowing: the band comes first and
collapses to the reading plus the target's own standing alert; the three populations stay three
headings with three counts, because merging them here would be the one change that alters
meaning rather than layout; and every control clears 44px — the *Decide* button goes full width
because a decision with five outcomes is not a thing to tap at the edge of a row. The index
itself obeys the no-structure-from-data rule: five rows, always the same five, hollow zeros
included.

### Screen A · Connected systems

**Should it exist once a target is registered? Yes — and it cannot be the parent of the target
rows.** Structure never moves in response to data, so this screen cannot appear only when the
count is zero, and cannot disappear when the count is one. That settles the question by ruling
out the interesting answer: it is always in the rail, so it has to be worth its seat at one
target.

It is, but only if it stops being a table you pass through. **It is the registration surface,
not a directory.** It answers one question no target page can — *is this all we have, and what
registers another* — and it names what Syndra is consequently not doing. The per-target rows
stay siblings in the rail, because a target is a daily destination and a list of one is not;
making the index their parent would put a click in front of the page an operator actually
opens, in exchange for a hierarchy that is one level deep.

What it gives up to earn that: no per-target action, no counts an operator might act on, no
drill-down other than the name. Everything you can *do* to a target lives on the target's own
page. The index is where you find out that a target exists and how one comes to exist.

If it ever grows a button, the argument for keeping it collapses and the per-target rows should
absorb it.

### Screen C · beside Health, not beneath it

Beneath, the second card reads as more of the first — which is the exact conflation the two
cards exist to prevent. Side by side at 1440 they are two authorities; the phone stacks them
and relies on the headings. A failing disk surfaces here and nowhere else in the product.

### Screens D and E · both leave the band and go to region 1

This is the one place my structure contradicts the brief; the argument is in section 4.

### The mapping screen · one thing holds the top of it permanently

The working copy can differ from the newest published version, and that is the fact a table
row cannot say. So it is not a state the screen falls into — it is a band under the title that
always occupies the same seat and always says one of three things: *working copy matches
version 4*, *version 4 plus three edits nobody has published*, or *nothing has been published*.
Publish and Versions sit in that band and go inert rather than absent, which is what keeps the
band the same shape in all three.

**And when there are unpublished edits, the band lists them.** "Rolling back undoes work listed
nowhere" is only true while nothing lists it. Enumerating the three edits — each with its
author, its age and the number of people it moved — is what makes rollback a decision rather
than a gamble, and it costs four rows. Once it is listed, the whole reason this could not be a
table row on the target page is discharged.

The other thing that band has to say, and the thing an operator's intuition from every other
tool gets backwards: **unpublished does not mean not yet in effect.** Each of the three edits
landed through its own rehearsal and is already what Syndra converges against. Publishing does
not apply them; it records the set so a later rollback has something to return to. That is
stated twice on the figure.

**The plan is the artefact, not the form.** Edit and delete rehearse first — rollback does not
today, and that is section 4b. By the time anything is approved the form is gone: the edit is one line and the rest of the
panel is the three consequences, the third of which no form field implies — the accounts move
group, the files do not follow, and thirty-four people lose access to what the old group owns.

**The scope step is drawn as arriving, not as being there.** It is the backend's refusal
rendered, and its first sentence says so, so an operator who never crosses the cohort limit
never learns the step exists and one who does cannot mistake it for a form they filled in
wrong. Rung 3, with the *number* typed rather than the role name — the role name is in the
title and typing it back proves nothing, while 34 is the fact the ceremony exists to make
unmissable. The threshold is never named by the screen.

**A rollback rehearses as one plan for the version.** This is a recommendation, not a rendering
of what exists (section 4b). The argument is a number: one plan is the only shape that can
produce *71 distinct people*. Per-mapping rehearsals yield 61, 12 and 3, and nobody adding those
up gets 71, because five people hold two of the affected roles — so the figure the ceremony
exists to make unmissable is a figure only the whole-version plan can compute, and the plan's
last row says where the five went. Per-mapping would also fire the scope step three times for a
single act, which teaches an operator that the ceremony is a toll rather than a warning.
Unrehearsed, as today, is the only option that contradicts the screen's own argument: publishing
a set is rehearsed, and reverting a published set would not be.

The plan is not symmetrical and is not drawn as if it were — *gain* is lime, *lose* is red and
names dormancy rather than deletion, *move* is amber and repeats M3's file-ownership consequence.
Twelve people losing a share is the whole risk of a rollback, so it appears three times: in the
plan, under the typed number, and in the reason prompt. And the cohort count is people, never
role holders and never rows.

**Value validation is drawn as a pair, because the pair is the rule.** Fail-open on everything
except a definite no. The definite no is red, echoes the value in the field it came from, names
the two near-misses, and has no *Try again* — an operator handed a retry on a deterministic
refusal presses it twice before reading. The far more common case, the check that could not run,
is amber, allowed through, and spends its space on why that is deliberate and where the
consequence surfaces instead. Both cards say what state the mapping is in, because a validation
message that does not say whether the thing saved is the most expensive ambiguity on a screen
that moves access for thirty-four people.

**The census line and the screen's first paragraph carry the same two facts in the same words.**
*Two reach it*, and *changing one moves access for everybody holding that role*. An operator who
clicked because of one sentence should find that sentence at the top of what they clicked into,
or the click feels like it went somewhere else.

### B1 · a decision somebody else already took

Not red and not shaped like an error: accent, in the accent the decided-and-waiting badge
already uses, because what happened is that the finding *became decided* — a state the design
already has a colour for. The three-state block stays at the top and does not dim; it is still
true and the operator arrived to read it. Below it, the two answers are compared as a claimant
pair rather than as an error message — what you had picked, what now stands, each with its
consequence in the words the resolution list uses — so an operator can tell in one glance
whether they even disagree. Most of the time they will not, and that is the common case the
figure is written for. The other operator's reason is quoted in full rather than linked: the
mandatory reason exists exactly for the person who arrives second, and a link would put the one
thing they need one click away. The API's own sentence — a UUID, a snake_case resolution, a
subject id — appears nowhere.

### B2 · a lifecycle change that did not land

The refusal and the state are two separate blocks, in that order, and the state block still
leads with its lime dot and the word *Active*. The single most expensive misreading here is
that the page looks like read-only took, or like the state is now unknown; it is known, it is
what it was, and both blocks say so. The typed reason is echoed back rather than cleared — a
mandatory-reason field that empties itself on a network failure teaches people to type "asdf"
the second time. The breaker exemption is stated in the copy rather than left as behaviour: an
operator who presses this four times should know none of the four made the target look worse.
*Try read-only again* is the one honest retry on a degraded page, and it takes the amber outline
rather than a violet fill, because the page's violet belongs to *Reconcile now*.

### The member's side · three rules and one word I would not use

**The notice is never the first thing on the page.** A member arrives to get into their files.
The state of their access comes first and the pause comes second, because the pause does not
change the answer to what they came for. Reversing those two is the whole failure of this page.

**Nothing is dimmed and nothing is hidden.** Their credentials, paths and instructions stay
exactly as reachable as before. The moment the credential block dims, the page has said their
access is affected, which is false. Only what writes is held back, and only that says so.

**No estimate.** "Usually within the hour" is a promise Syndra cannot keep, and a member told an
hour who waits three has been misled by the page rather than by the drain. What replaces it is
what *will* happen — nobody has to come back and check — and, for read-only, a person: whoever
runs the makerspace can see the pause and can lift it. That is the honest escalation and it
costs Syndra nothing it cannot keep.

**The word I would not use is *maintenance*.** It is the operator's word for a state they chose;
to a member it means the thing they are trying to use is unavailable, which it is not. *Paused*,
and what is paused, throughout. *Draining* does not appear either.

C1 and C2 are structurally identical and differ by two clauses, deliberately — a member should
not have to learn a second page shape because an operator picked a different state. Draining
says "a few minutes"; read-only says "while we work on the file server" and puts the
deliberateness on the surface, because a member reading that changes are paused with no end in
sight concludes something is broken. The badge changes with the sentence: *shortly* attached to
an open-ended pause is the small lie that makes the rest of the page untrustworthy.

C3 is the only figure where the pause sits *inside* the state card rather than below it. The
thing they are waiting for is exactly the thing that cannot happen, so the two facts are one
fact, and separating them would let somebody read the accent half and miss it. The existing copy
is kept word for word — *recorded, not created yet, nothing needed from you* — because it is
still true and it is what makes the wait ordinary rather than personal.

---

## 2 · The eleven panels of `REFERENCE-current-page.md`

Every panel in the reference, and what the drawing does to it. Nothing is unmentioned; nothing
else was removed. All quoted copy is kept verbatim except where a row says otherwise.

| # | Panel | Disposition | Where it now lives | Why |
|---|---|---|---|---|
| 1 | Health | **Changed** | Band, left card, renamed *Syndra's reach* (T1–T4, S11) | All nine readings, the definition list and the freshness line kept in the same order, transport-secret-unreadable still above reachability. Two things change: the two findings that render inside it leave (see 9), and in their place it carries one red line saying something below needs a person before this card can be trusted. |
| 2 | What the target reports | **Changed** | Band, right card, beside Health (T1–T4, S9–S10) | Drawn for the first time. Placed beside rather than beneath, because beneath it reads as a continuation of Syndra's own card. The `degraded` list becomes a sentence at the top naming which of the four could not be read — a gap where an alerts row would be reads as "no alerts". |
| 3 | What roles reach here | **Removed** | Off this page. A census line stays in region 2 (T1–T4); the governing mapping and holder count also appear inline on a group finding (S5) | It was designed as a mapping screen and should be one. Editing a mapping moves access for everybody holding that role; a fact with that reach has no room to breathe in a table row. |
| 4 | Published versions | **Removed** | Goes with panel 3 | The working-copy-versus-published distinction is a sentence, not a row: "current version 4" meaning version 4 plus three unpublished edits, where rolling back undoes work listed nowhere. |
| 5 | People with an account here | **Kept, moved** | Region 2, headed *Bound* (T1–T4) | Person, account name, uid, how long bound; Hold then Take away, reversible first; the release-and-forget sentence kept as the card footer. One addition: a row carries a count of the findings that name that person, as a pointer up to region 1 rather than a status on the row. |
| 6 | Accounts nothing explains any more | **Kept, moved** | Region 2, headed *Lost their reason* (T1–T4) | Drawn at zero on all four page figures, keeping its seat with a hollow count. The grouped-by-cause behaviour and the refusal to act on anybody still a member are stated in the empty state rather than lost with the rows. Its populated form and bulk action are §29 and are not redrawn. |
| 7 | Accounts Syndra did not create | **Kept, moved** | Region 2, headed *Not created by Syndra*, third population (T1–T4) | "Reported, never triaged. These are not drift." and "Nothing on the account changes now; the next convergence applies their entitlements to it." are verbatim. Adoption blocked while the read is stale with the reason in the row; `root` listed and named not adoptable. Only change: the amber freshness banner inside this panel becomes the region's one freshness strip. |
| 8 | What it can do | **Kept, moved** | Region 3, beside Reconciliation, under *What runs here* (T1–T4) | Id, scope, whether it stops and asks, which parameters are never logged, disabled-with-reason rather than omitted. One addition: during an outage every row states "cannot run" with its reason, because the manifest is Syndra's own copy and is one of the few things still worth reading. |
| 9 | Waiting on a decision | **Changed** | Region 1, second on the page, expanded to hold the whole category (T2, S4–S8, S11, S13) | Moves from two-thirds down the page and becomes the home for the merge findings, the edited change record and the two records disagreeing about an account. Each row gains the three-state block. The still-standing reading becomes a soft accent badge, *Decided, waiting*; *resolved*, *done* and the tick do not appear. |
| 10 | Reconciliation | **Kept, moved** | Region 3, beside the operations it calls (T1–T4) | "The scheduled sweep runs every six hours" and "reads the target and queues what is already owed. Queueing is not applying." are verbatim, the second set as body copy rather than a hint. Carries the page's single violet fill. |
| 11 | Maintenance | **Changed** | Band, footer strip (T1, T3, T4) | Moves up because the state somebody set changes what the readings above it mean. Three states, the explanation carrying the weight rather than the labels, mandatory reason. One change with consequences: drawn as still working while the target is not answering, on the grounds that it edits Syndra's own record — an exception to "degraded: every action inert", and an open question in section 5. |

No panel is unlisted. If a panel exists in the deployed page that is not in the eleven, I did
not see it and have not drawn it.

---

## 3 · What I changed my mind about between the brief and the drawing

**I expected to keep four questions and add a fifth.** Drawing the failure case is what changed
it. On a target that is not answering, *is it answering* and *what state has somebody put it in*
are unreadable apart — draining plus silent is a different fact from silent, and they were six
panels away from each other. They collapsed into one band, which left three regions and not
five.

**"Nine of the eleven panels have nothing to say" — I do not think that is true, and the figure
depends on it not being.** Six of them still have something to say when the NAS is silent: the
reach card, the mapping census, the manifest, the maintenance state, and both Syndra-side
findings. What goes quiet is only what was read *from* the target — and the last state read is
content, so those keep their rows, dimmed and dated. That is the whole reason T3 is not a
column of empty cards, and it is a reframing of the brief rather than a solution to it.

**The fresh deployment turned out to be the hard figure.** I expected the outage. But three
hollow zeros and one populated list is the state that most easily reads as broken, and the only
thing that fixes it is every empty state naming who it waits on — a person's decision, a
schedule, or a consequence of the first two. None of them says "nothing to do", and none of
them is an error.

**The tab constraint broke differently than I expected.** I assumed a tabbed desktop would fail
"nothing appears or disappears with its count" outright. It passes it — four tabs, always four,
whatever the data. It fails on the next move: the only way to make a finding under an
unselected tab visible is a badge on the tab, which is data driving structure by another route.
So the argument against tabs is not the letter of the rule; it is that the rule's purpose
cannot survive them.

**One label rewritten.** "What the target last reported" became "It used to be". The first names
where a value came from; the second is the question an operator actually asks, and for most
targets there is no history anywhere else that answers it. The mono `base` · `ours` · `theirs`
sit under the three panels because those are the words in the payload and in a support
conversation.

**Deciding twice: I had it backwards, and the backend's reason is better than mine.** S7 said
"deciding again replaces the queued work; it does not stack." It fails closed instead, and for
`unbound` a replacement would release the account on the target while a re-provision sat in the
outbox. So the row's promise changes kind: from *you can change your mind* to *the first
decision wins, and if somebody else got there first you will be told whose it was*. That is a
promise about being informed, which is worth nothing without B1, which is why B1 exists.

**Maintenance surviving an outage was right for the wrong reason.** I argued it edits Syndra's
own record. It does not — it is `POST {addon}/lifecycle` over the network and it can be refused.
It survives because that call is deliberately exempt from the breaker, so that a refusal cannot
be confused with the target having stopped answering. The consequence for the drawing is that
it needs a failure path, which no figure had; B2 is that path, and the exemption is now stated
in the copy rather than left as behaviour.

**The unexplained-access screen changed shape once I led with *applied*.** Two of the three rows
have a full history, which means the operator's question is no longer *where did this come
from* but *who undid it, and did they mean to*. That produced two labels the brief does not
contain — *Apply it again* and *Stop wanting it* — and left the third row as the only genuinely
unexplained one, keeping the drift screen's own revoke-and-adopt pair.

---

## 4 · What the brief got wrong

**D and E are placed inside the Health card, above the reachability reading.** The priority is
right and I kept it; the placement is wrong. The edited change record and the ownership
disagreement are not health readings: neither is a fact about whether Syndra can reach the
machine, both stand whether or not it answers, and both wait on a person — which is the
definition of region 1. A reader who meets them in a list of readings skims them in the same
rhythm as *in flight: 0*. They moved to region 1, and the band keeps a red line pointing down at
them so they are still the first thing read (S11).

**C is placed directly beneath Health.** Beneath, the target's own readings read as more of
Syndra's — the one conflation those two cards exist to prevent. They are side by side at 1440
(S9–S10); the phone stacks them and relies on the headings.

**"Nine of the eleven panels have nothing to say" during an outage** overstates it, and the
degraded figure depends on that being wrong. Section 3 has the count.

Nothing else in the brief looks wrong to me. The forbidden list, the confirmation rungs, the
freshness rules and the queued-is-not-done rule are all load-bearing in these drawings, and
where I bent one I have said so.

---

## 4a · What commission 3 changed on the earlier boards

Four things. The boards in this zip are the corrected ones; the figure ids are unchanged.

**The dashed disabled control is gone.** In this codebase dashed already means *produced by an
automatic rule* — a dashed chip on a role, a dashed edge in the access map. It is a provenance
idiom, so "off right now, with a reason" cannot borrow it: a dashed *Adopt* would read as
*adoption is automatic here*, which is the opposite of what it says. Every one of them is now
the disabled control at reduced alpha with its reason as body text on the line beside it or in
the card footer. Affected: T1, T3, T4, S5. The one remaining dashed border in the package is on
M6, where it marks a figure boundary rather than a control, and the caption says so.

**The pool warning is the target's own flag.** `pool.query` returns `warning` alongside
`healthy` and `status`, and Syndra passes it through, so S9's row now renders that flag and
names whose it is. It was amber on arithmetic over free-over-size, which would have been Syndra
publishing a conclusion the target declined to publish — the exact conflation the two-card split
exists to prevent, and in the row indistinguishable from a flag the NAS actually raised. The
closing card on that board is corrected too; it used to offer this as a decision to overrule.

**S7's promise inverted.** The row no longer says deciding again replaces the queued work. It
says a finding takes one decision, gives the reason, and points at B1.

**One addition is not an addition.** Section 6's freshness strip already exists as
`ReadFreshness`, with the tone dot, the age sentence, the truncation clause and the stale-only
*Read again*. The drawing is unchanged and the entry is now marked as an existing component. I
had considered the amber banner inside the unmanaged inventory and had no way to see the shared
component above it.

---

## 4b · A drift between two file headers and the routes

Two file headers in the repo assert that edit, delete and rollback all rehearse first. Only two
of the three do: `rehearse-edit` and `rehearse-delete` exist, and
`versions/{version}/rollback` has no rehearsal — it restores the version and queues a
convergence per affected holder, unrehearsed, with no cohort acknowledgement. The comments are
not a specification and the drift is pre-existing.

M7 draws the shape I am recommending rather than the behaviour that exists, and its caption says
so in its first sentence. M1's footer points at M7 rather than claiming rollback rehearses
"like any other change", which was the sentence that carried the drift onto the board.

---

## 5 · Open questions

Ten of the thirteen from the last commission are settled by the code and the answers are folded
into the drawings. Two were wrong in ways that changed a figure and are written up in section 3.
What remains:

**Still open, and not mine to settle**

1. **What routes sit behind *apply it again* and *stop wanting it*** (S15). The two labels follow
   from leading with *applied*: the question stops being where this came from. I did not assume a
   route for either, and the drift screen's own revoke-and-adopt pair may already be the answer
   under different words.
2. **Which readings on the Connected systems index can co-occur** (S2). A target could plausibly
   be backed off *and* have an unreadable transport secret. I drew one reading per row, which
   forces a precedence I invented; the Health card has the same question with nine readings.

**New, from this commission**

3. **Publishing needs a note field, and it is not the rehearsal's.** Every version in M1 carries
   a sentence somebody wrote about a *set* — "archive admins split out of fabrication" — which is
   a different kind of statement from the reason attached to a single edit. Publish is drawn as
   one violet control in the band; the note it demands is a step I have not drawn, and it is the
   only ceremony on that screen that is not a rehearsal.
4. **Whether a rollback should rehearse at all is a product decision, and M7 is my answer:**
   one plan for the version, rung 3 once, cohort counted in people. It is drawn as a
   recommendation and section 4b says what exists instead. What I still cannot answer is whether
   the cohort limit is *meant* to apply to a return to a published state; I assumed it does,
   because twelve people lose a share either way and where the instruction came from does not
   change that.
5. **Does the 71 exist as a number the backend can produce?** The plan needs distinct people
   across three role cohorts, deduplicated. If the only available count is per-mapping holders,
   the figure's central claim is undrawable and the honest fallback is to show the three counts
   and refuse to total them — which weakens the ceremony rather than faking it.
6. **Who is "whoever runs the makerspace"?** C2 and C3 send a member to a person and C3 gives it
   a control. Syndra knows who holds the operator role; whether that is a name, a room, or a
   mailto is a question about this deployment rather than about the design.
7. **Is a saved-but-unapplied password held anywhere a member can see it was held?** C1 and C2
   promise "we will apply it as soon as changes resume" and "you do not need to come back". If
   that queued credential can fail later, the member's page needs a state for that, and I have
   not drawn one.

## 6 · What the drawings add to the design system

Six things that are not in the tokens I was given. No new spacing step and no new radius:
everything is 60px rows, 14/16/20px pads, 14/18/20px radii. The touch index rows are 48px,
which is the dense row inside the 44px floor, not a new step.

### Three-state block — S4, S5, S6, T2

Three inner blocks in a row — *it used to be*, *Syndra wants*, *the target holds now* — with the
mono merge word under each and a tint on the sides that moved. It exists so that *which two
agree* is legible before any word is read.

*Considered first:* the definition list Health already uses for last answered / in flight /
lifecycle. It has no way to mark one term as the one that differs, and three values read as a
list lose the comparison, which is the entire content of the row. A two-column diff was the
other candidate; there are three states here, and the third is the one nothing else in the
product could show.

### Count chip, three forms — every region heading

A pill in a heading that holds one seat and says one of three things: a filled number, a hollow
outlined zero, or an em dash when the read failed. It is what lets a region keep its place at
zero without a reader mistaking a failed read for an empty one.

*Considered first:* the existing badge. Badges name a thing — *accent*, *amber*, a word in a
pill. This one has to carry *none* and *unknown* in the same box without changing size, which is
a different job, and the hollow form borrows the dashed-and-hollow language §29 already uses for
"there is deliberately nothing here".

### Freshness strip — T1–T5, S1 · **exists already as `ReadFreshness`**

One row: a tone dot, a sentence carrying an age, an optional clause when the read hit its cap,
and a *Read again* that exists only when the read is stale. Neutral ground at live and ageing;
amber ground at stale and provisional. Used twice on the target page, for the health snapshot
and the account list, and nowhere else — no other age is written on the page.

Not an addition. The component is `ReadFreshness` and it already carries all four parts; the
drawing matches it and needs nothing new. Left in this section because the reasoning is worth
having written down, and because I reached it independently, which is mild evidence the
component is shaped right.

*Considered first:* the amber banner inside §21's unmanaged inventory, which is where I could
see this behaviour living. It is a warning band, and four of the five freshness states are not
warnings; wearing amber at *read just now* would spend the colour on the good case. There is a
shared component above that banner and I had no way to see it.

### Claimant pair — S13, S14

Two panels, identical in ground, border, type and order of facts, each carrying the provenance
of its claim, neither pre-selected and neither called correct. For the case where two of
Syndra's own records disagree and the design has no basis for a default.

*Considered first:* the product's existing dialog shape, which pairs a recommended action with a
quieter one. Every instance of it has a preferred answer. Here a preference would be the design
deciding the thing the copy says it cannot decide.

### Neutral reading dot — S2

A sixth dot tone, `rgba(243,245,239,.42)`, for a reading that is a runtime fact and not a state
of health: *registered, no manifest read yet*.

*Considered first:* amber. Amber is a deadline or a broken assumption, and an add-on that has
not answered yet is the ordinary first minute of its life — colouring it would teach an operator
that the colour does not mean anything. Lime was the other candidate and is worse: nothing is
healthy here, nothing has been read.

### Version state band — M1, M2, M5

A band under the title holding one seat and three readings: *working copy matches version N*,
*version N plus K unpublished edits* with those K enumerated beneath it, and *nothing has been
published*. Publish and Versions live in it and go inert rather than absent. It is the whole
reason the mapping set could not stay a panel on the target page.

*Considered first:* a badge beside the title, and a banner above the rows. A badge cannot hold
three enumerated edits, and once they are not enumerated the screen is back to hiding the thing
it exists to show. A banner would have been right if the clean state had nothing to say, but it
does — *working copy matches version 4* is the sentence that makes the other two readable, and a
banner that appears only when something is wrong cannot deliver it.

### Rehearsal panel — M3, and behind the add form

A panel that replaces a form with a plan: the edit as one line, then the consequences as counted
lines with tone dots, then the freshness of the read the plan was built from. Approving approves
the plan. A stale plan is rehearsed again and shown again rather than sent.

*Considered first:* the confirmation dialog at rung 2, with the counts in its lede. The counts
are three separate facts here and one of them — files staying with the old group — is a
consequence no operator infers from a diff, so it needs a line of its own rather than a clause.
A dialog also cannot host the scope step without becoming a dialog inside a dialog.

### Region index — T5, touch only

A card at the top of the phone column listing the page's regions in page order, each with its
count in the row, each a jump. Always the same rows, hollow zeros included. It is what the four
horizontally scrolling tabs become: a way to skip a region rather than a way to hide three.

*Considered first:* tabs, as the touch board drew them, and a sticky region header. Tabs put a
finding behind an unselected state and then need a badge to compensate; a sticky header tells
you where you are but not what else exists, which is the question a four-region page raises.

---

## 7 · Described but never drawn

These exist in this reply only. There is no figure for them, and they must not be read as
handed-over drawings.

- **The add-a-mapping form.** M5 offers *Add the first mapping* and M1 offers *Add a mapping*.
  The form is the rehearsal panel in M3 with a role picker and a value field in front of it, and
  the field picker's single option is explained by the card in M1. I would rather you saw the
  rehearsal and the two refusals than a form.
- **The publish step and its note field.** Open question 3. Publish is a control in the band on
  M1, M2 and M5; what it opens is not drawn.
- **The delete rehearsal.** `rehearse-delete` exists and is not drawn; it is M3 with a plan that
  only removes. The rollback rehearsal is drawn in M7 as a recommendation, not as existing
  behaviour.
- **The mapping screen's own degraded state.** M4's right-hand card shows what an unreadable
  target does to one validation. What the whole screen looks like when the target has not
  answered for forty minutes — which rows dim, whether Change stays live, whether the census
  count is still true — is not drawn, and by the logic of T3 most of it should stay live because
  mappings are Syndra's own record.
- **The mapping screen on touch.** T5 establishes the region index for the target page. This
  screen has a version band, a wide table with holder counts, and a rehearsal panel, none of
  which I have narrowed.
- **The member page's no-entitlement state under a pause.** C3 covers entitled-and-waiting. A
  member with no entitlement has nothing paused and should presumably see nothing, but I have not
  drawn it to confirm that.
- **The *backed off* reading.** Described in the footer of S2 and in T3's caption, in words,
  because the deployment I drew has no target in that state. It is red and it sends an operator
  to the Syndra host rather than the add-on.
- **The populated *Lost their reason* card and its bulk action.** §29 has them; every figure
  here draws that region at zero.
- **The member side of a withdrawal.** C1–C3 cover a pause. What a member reads when access is
  actually taken away is a different page and a different sentence, and it is not here.
- **The populated *Not created by Syndra* triage flow past *Adopt*.** The row and its
  consequence sentence are drawn; the adoption confirmation is not.
- **The scope step behind a blast-radius refusal.** Named in section 1 as a backend refusal
  producing a step, per the build notes. Not drawn.
