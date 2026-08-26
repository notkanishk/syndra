# Design reply: the add-on target plane

Everything I said in the course of this commission that is not drawn on a board. Written as
the design authority for the two boards in `design/`. Figure ids referenced here (`T1`, `S4`)
are the ids stamped into each figure's caption.

Boards:

- `design/Syndra Target Page.dc.html` — the redesigned target page. Figures T1–T5.
- `design/Syndra Target Screens.dc.html` — the six supporting screens. Figures S1–S15.
- `design/Sidebar.dc.html` — the existing rail component, included so the boards open
  standalone. Unchanged from the handoff bundle.

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

## 5 · Open questions

Guesses are drawn, and each one is a question. Several are about the payload rather than the
design, and the drawing assumes an answer I have no way to check.

1. **Is one control staying live during an outage acceptable?** Maintenance is drawn as working
   while the target is not answering, because it edits Syndra's own record. Rule 4 says every
   action in a degraded state is inert. Which of the two wins?
2. **Do the health snapshot and the account list have separate ages?** I drew two freshness
   strips on the grounds that they are two reads. If `state_read_at` is one value for the whole
   target, one strip is the honest drawing and the second is a second way of saying how old
   something is.
3. **Where do the log-anchor and binding-conflict findings come from?** Only their resolve
   endpoints are named. I drew both as arriving with the health read, which is where they render
   today — if they have their own reads, does moving them into region 1 change what the page
   fetches?
4. **Should the census line name the two mappings rather than count them?** Two would fit. Five
   would not, and the line would then be the only thing on the page that changes shape with its
   data — which is the rule it would be breaking.
5. **Does the dashed control now carry two meanings?** The system uses dashed for "no endpoint
   yet". I also used it for "off right now, with a reason" — adoption during a stale read,
   deciding during an outage. If those need to look different, which one keeps the dashes?
6. **Is the pool at 94% Syndra's conclusion to draw?** It is amber on arithmetic the target
   published without raising an alert of its own — the only place in these drawings where Syndra
   says something the add-on did not. If it stays, is the threshold Syndra's or configured per
   deployment?
7. **What routes exist behind "apply it again" and "stop wanting it"?** The two labels follow
   from leading with *applied*. I did not assume a route for either, and the drift screen's own
   revoke-and-adopt pair may already be the answer under different words.
8. **Does deciding a finding twice replace the queued work or add to it?** The
   decided-and-waiting row says "deciding again replaces the queued work; it does not stack".
   That is a guess about the backend, written as a promise to the operator.
9. **Is the half-finished unbind repaired through the same resolve call?** I drew *Finish it* as
   idempotent and said so in the row — "pressing it again changes nothing on the target". If it
   is a different call, the copy is still what the row needs to say.
10. **Which readings on the index can co-occur?** A target could plausibly be backed off *and*
    have an unreadable transport secret. I drew one reading per row, which forces a precedence I
    invented; the Health card has the same question with nine readings.
11. **On a fresh deployment, what does a sweep with nothing owed return?** The card says it will
    read the target, owe nothing, queue nothing, and say so. That needs a response that
    distinguishes *read and owed nothing* from *did not read*, and I do not know that it does.
12. **Does a finding always name a person?** The roster rows carry a count of the findings that
    name that person, which assumes every finding maps to a subject. A difference on an
    unmanaged account, or on a group with no binding, would have nowhere to point.
13. **Is maintenance-stays-live consistent with what the member sees?** If an operator sets
    read-only during an outage, the member's page needs to say something, and I have not drawn
    the member side.

---

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

### Freshness strip — T1–T5, S1

One row: a tone dot, a sentence carrying an age, an optional clause when the read hit its cap,
and a *Read again* that exists only when the read is stale. Neutral ground at live and ageing;
amber ground at stale and provisional. Used twice on the target page, for the health snapshot
and the account list, and nowhere else — no other age is written on the page.

*Considered first:* the amber banner inside §21's unmanaged inventory, which is where this
behaviour currently lives. It is a warning band, and four of the five freshness states are not
warnings; wearing amber at *read just now* would spend the colour on the good case.

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

- **The mapping screen itself.** Section 1 says what it takes with it from the target page — the
  rows, the fields, Change and Remove, the published versions, the working-copy distinction —
  and does not draw any of it.
- **The *backed off* reading.** Described in the footer of S2 and in T3's caption, in words,
  because the deployment I drew has no target in that state. It is red and it sends an operator
  to the Syndra host rather than the add-on.
- **The populated *Lost their reason* card and its bulk action.** §29 has them; every figure
  here draws that region at zero.
- **The member side of anything.** No figure shows what a member reads when access is withheld,
  including the case in open question 13.
- **The populated *Not created by Syndra* triage flow past *Adopt*.** The row and its
  consequence sentence are drawn; the adoption confirmation is not.
- **The scope step behind a blast-radius refusal.** Named in section 1 as a backend refusal
  producing a step, per the build notes. Not drawn.
