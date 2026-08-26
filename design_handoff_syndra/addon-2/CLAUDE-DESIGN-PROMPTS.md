# Claude Design prompt pack — the target plane, commission 2

Read `README.md` in this folder first. It says what already exists and why this
is not a restyle.

## How to use this

1. Paste **Prompt 0 from `../mobile/CLAUDE-DESIGN-PROMPTS.md`** first, on its
   own. It is the design system and it has not changed. Do not paste a second
   copy of it from here.
2. Then **Prompt A**. Everything else inherits its structural answer, and
   drawing B–H first means drawing them twice.
3. B–H can be pasted in any order after A.
4. Desktop is the primary canvas for this commission (1440 × N, the rail at
   252px). Touch figures only where a prompt asks for them.

## Decisions already taken — state them, do not re-open them

| Decision | Ruling |
| --- | --- |
| Board vs codebase | **The codebase wins.** It is deployed and in use. Where §21 and the built page disagree, the built page is the fact and the board is the intent. |
| Colour | Tokens only. `ui/src/app/globals.css` is the palette and a raw hex fails the build. Both themes are authored in full; draw dark, and say what changes in light only where it is not mechanical. |
| Structure never moves in response to data | A section with nothing in it keeps its seat and shows a hollow zero. This is the rule the navigation and every list already follow. Do not draw a panel that appears when its count goes non-zero. |
| Freshness | `ReadFreshness` exists and is one component: a dot, a sentence with an age, an optional truncation clause, and a `Read again` that appears only when the read is stale. Reuse it. Never invent a second way to say how old something is. |
| Confirmation | Three rungs, already specified in §31. Rung 1 a plain button, rung 2 an acknowledgement with the number inside the sentence, rung 3 type-the-name. Size the ceremony to the act. |
| Controls | One surface per control: `Button` (variants `accent` / `outline` / `ghost` / `danger` / `dangerConfirm`), `Badge`, `Tabs`, `FilterPills`, `Card`. A row's own action is the outline pill; `ghost` is only the quieter half of a pair; solid red appears only on a dialog's confirming button. |
| Copy | Plain sentences that say what is true. Never "done", never a tick, never a word where an age belongs. Say which machine to look at. |

---

# PROMPT A — the target page, as a composition

**This is the commission. The others are pages; this one is an argument.**

`/system/targets/{target}` · Advanced › System › ‹target›

Two existing boards drew this page as **four questions in this order**: is it
answering, whose accounts are on it, what can it actually do, and what state has
somebody put it in. §21 stacked them; M20 made them four horizontal tabs with the
freshness strip above.

It now carries **eleven panels**, and I need you to decide what it is.

## What is on it, and what each one answers

1. **Health** — five readings, each naming which machine to look at: not
   answering (the add-on), backed off (this host, not the target), draining or
   read-only (somebody chose it — accent, never amber), still settling (`n` calls
   issued before the drain have not returned), serving from a mirror with an age.
   Plus three more added since: an unreadable transport secret, an edited change
   record, and two records disagreeing about ownership. The last two are
   *findings* — they have their own prompts, F and G — and they render inside
   this card, above the reachability reading, because each one *explains* a
   target that will not answer.
2. **What the target reports** — the NAS's own health. Alerts with levels, pools
   with capacity, services running or stopped, hostname and version. A failing
   disk surfaces here and nowhere else in the product. Prompt D.
3. **What roles reach here** — one row per mapping: project, role, field, value,
   how many people hold that role, Change, Remove. Editing one moves access for
   everybody holding that role.
4. **Published versions** — a snapshot of (3) with a mandatory note, and a
   rollback per version.
5. **People with an account here** — the managed roster. Per row: person,
   username, uid, since when, Hold, Take away. Prompt E.
6. **Accounts nothing explains any more** — accounts Syndra created and no longer
   has a reason for. Refuses to act on anybody still a member. (§29 covers this.)
7. **Accounts Syndra did not create** — the unmanaged inventory. Never drift.
   Adoption blocked while the read is stale. (§21 covers this.)
8. **What it can do** — read from the add-on's manifest. Each operation, its
   scope, whether it asks for confirmation, which of its parameters are never
   logged, and — when it cannot be performed — the reason, shown disabled rather
   than omitted.
9. **Waiting on a decision** — differences reconciliation refuses to resolve.
   Prompt C.
10. **Reconciliation** — "the scheduled sweep runs every six hours", and a
    Reconcile now that reads the target and *queues* what is owed. Queueing is
    not applying.
11. **Maintenance** — active, draining or read-only, with a mandatory reason
    because the person who reads it next is not the person who set it.

## The question I want answered

Eleven stacked cards is a scroll, not a structure. An operator arriving with a
specific question — *why can this person not get in?* — has no route to it.

**Do the four questions survive?** Either:

- **They do**, and panels 2–11 belong under them: Health absorbs 2 and its
  findings; "whose accounts" absorbs 5, 6 and 7; "what it can do" is 8 and 3 and
  4; "what state is it in" is 10 and 11 — and 9 is either a fifth question or
  belongs under the second. Show me the grouping and defend it in the caption.
- **Or they do not**, and the honest structure is different now — because the
  page acquired a whole second category the boards never anticipated: *things
  that need a person*. Findings, decisions, disagreements. Argue for whatever
  replaces the four.

Whichever you choose, it has to answer:

- **Where does urgency live?** Three panels can carry work that is waiting on a
  human (9, and the two findings inside 1). Today already surfaces those counts.
  A panel that appears only when non-zero is prohibited — so how does a page
  whose eleventh panel is quiet look different from one where somebody's access
  is disputed?
- **Does this become tabbed on desktop, like M20 is on touch?** If it does, say
  what happens to a finding that lives under a tab nobody is looking at. If it
  does not, say why the desktop can carry what the phone could not.
- **Which panels are not, in fact, this page's job?** (3) and (4) were drawn in
  §24 as a mapping *detail screen* with version history and a blast-radius
  acknowledgement, and shipped as two panels here instead. Say whether that was
  right.

## Draw

- The whole page at 1440, in whatever structure you land on, with real content.
- The same page with three findings outstanding, so I can see how urgency reads.
- The same page for a target that is **not answering** — which is when an
  operator most needs it and when nine of the eleven panels have nothing to say.
- One 390 × 844 figure reconciling your answer with M20's four tabs.

---

# PROMPT B — Connected systems

`/system/targets` · `GET /api/v1/targets` · Advanced › System › Connected systems

An index of what the deployment has registered. It exists because a deployment
that has registered *no* add-on previously showed nothing about add-ons anywhere
— no row, no page, no sentence — and an operator reads that as the platform not
having shipped rather than as it not being configured. That empty state is the
reason this screen exists, so draw it first and draw it best.

**Per target:** its name, its id, one reading of whether it is answering, and how
many operations it offers. The reading is four states and they are not
interchangeable — registration is deployment configuration, a manifest having
been read is a runtime fact, a transport secret that will not load is a fault on
*this* host, and a suspended breaker is Syndra refusing its own calls. Each sends
an operator to a different machine. A target that has published no manifest shows
no operation count at all, because `0` there is a claim about the target rather
than about Syndra never having asked.

**Empty:** no system is connected, what an add-on is in one sentence, and what
registers one (`ADDON_TARGETS` in the deployment's environment, then its
container). Also what is *not* happening: Syndra is governing identity only, and
nothing outside the identity provider is being reconciled.

**Draw:** one target answering; one registered and never answered; one with a
broken transport; the empty state; the error state.

**Also answer:** should this screen exist at all once a target is registered, or
should it be the place the per-target rows live rather than a peer of them? The
navigation currently shows both the index and a row per target.

---

# PROMPT C — Waiting on a decision

Target page · `GET /api/v1/targets/{t}/merge-findings` ·
`POST …/merge-findings/{id}/resolve`

Reconciliation is a **three-way merge**. It knows what the target last reported
(base), what Syndra wants (ours), and what the target holds now (theirs). Most
differences resolve themselves. Three do not, and they land here.

- **theirs_only** — Syndra did not change it, so somebody changed it on the
  target.
- **conflict** — both moved, differently.
- **deleted_upstream** — the account is gone.

**Every row must say what it used to be.** That is the question an operator asks
first, it is what no surface could answer before the merge base existed, and a
row without it sends somebody to a target history that for most targets does not
exist.

**The decision** is one of five, each with a mandatory reason: keep Syndra's,
take the target's, provision it again, stop managing it, or record that the two
sides now agree. Taking the target's value is **only offered when there is a
per-subject home for it** — a group is produced by a role mapping that reaches
everyone holding that role, so "just for her" is not a thing that exists. When it
is not offered, the row says why, and names the mapping that governs the value
along with how many people hold that role.

**A decision is not a settlement.** Deciding queues work; the row closes when a
later pass sees the target agree. So a decided row is still standing, and must
read as *decided and waiting* rather than as resolved — saying "resolved" there
is the surface claiming a difference is over while it is still on the target.

**One repair case:** a "stop managing it" whose release reached the target but
whose follow-up failed leaves a half-done unbind. That row offers to finish, and
says pressing again changes nothing on the target.

**Draw:** the three outcomes; an adoptable conflict and a non-adoptable group
finding side by side; the decision form open; a decided-and-waiting row; the
repair row; and the empty state, which is good news and should look like it.

---

# PROMPT D — What the target reports

Target page · `GET /api/v1/targets/{t}/system-health`

Directly beneath Health, and the distinction is the whole point: the card above
answers *is Syndra able to talk to it*, this one answers *and is the machine
itself all right*. Two authorities, two failure modes, never one chip.

**Carries:** alerts (level, class, the target's own prose, when, whether
dismissed); pools (name, status, healthy or warning, free/allocated/size);
services (name, state, enabled); hostname, version, uptime. Plus a `degraded`
list naming which of those four could not be read — "nothing is wrong" and "I
could not look" must never render the same.

The live deployment has one standing alert: an uncorrectable error on `/dev/sde`,
open since July. Draw that. It is not a Syndra problem and it is the first thing
this surface was built to show.

**Draw:** healthy; one warning alert; a pool at 94%; a partial read where pools
answered and alerts did not; the whole thing unreadable.

---

# PROMPT E — People with an account here

Target page · `GET /api/v1/targets/{t}/inventory`

The **managed** half of "whose accounts are on this target", which §21 never
drew — it drew only the accounts Syndra did *not* create. It reads above the
unmanaged list because it is the half an operator acts on, and the unmanaged one
then reads as *and what else is here*.

**Per row:** the person (a name, never an id), their account name, uid, how long
it has been bound, and two actions in this order — **Hold** (pause what a role
grants, without touching the role) then **Take away**. Reversible first.

**Also:** a binding whose account no longer exists on the target can be forgotten
— Syndra stops managing it, nothing is deleted, and it can be bound again by
adopting the account if it comes back. That path has a confirm step and a failure
message that names the repair rather than saying it worked.

**Draw:** a populated roster; the forget-this-binding confirm; a release that
came back unconfirmed; nobody bound at all — which is the live deployment's
current state and is not an error.

---

# PROMPT F — The change record has been edited

Inside Health · `POST …/log-anchor/resolve`

The add-on keeps a hash chain of every mutation. Syndra anchors what it last saw:
a head and a record count. When the target reports **fewer records** than the
anchor, or the **same number hashing to something else**, that is a chain that
has been edited — and a chain verifies its own contents and cannot notice its own
truncation, so this is the only thing that can.

The anchor does not advance past a finding. It stays until a person resolves it,
and resolving means **re-baselining to a cited head** — not dismissing. There is
no state where this is acknowledged and the anchor is still frozen, because that
state detects nothing and reads as handled. The head is cited in the dialog
rather than read at the moment of the click, because a chain that changed again
while the dialog was open is exactly the event this exists to notice.

The records that went missing stay missing.

**Draw:** both violation reasons; the resolve dialog with its cited head and
mandatory note; how it sits above the reachability reading.

---

# PROMPT G — Two records disagree about who owns an account

Inside Health · `POST …/binding-conflicts/{id}/resolve`

A change for one person was applied to an account, and Syndra's own binding says
the account belongs to somebody else. **Both records were written by this system
and neither is authoritative** — which is why it needs a person and not a
reconcile, and why the design must not call either one correct.

It renders above the reachability reading, because a target being down is
temporary and this is not: it stands whether or not the add-on is answering.

Resolving moves the account between people **in Syndra's records and touches
nothing on the target**. That distinction is the whole copy problem: an operator
who reads it as "fixed" will not converge afterwards.

**Draw:** the finding with both claimants named and neither preferred; the
dialog; what it says about what has and has not changed.

---

# PROMPT H — The applied history on an unexplained grant

`/governance/drift` · `GET /api/v1/governance/drift/{id}/origin`

A grant that Syndra applied, and that somebody then removed by hand in the
identity provider, is currently listed as an independent mystery with no history.
It is not. Syndra remembers proposing it, applying it, and the target accepting
it.

**The story a row can now tell:** who granted it and why, when Syndra applied it,
who the identity provider says removed it, and when it was last seen. Lead with
*applied* — the fact that Syndra did this is what distinguishes it from a grant
nobody can account for, and it changes the operator's question from "where did
this come from" to "who undid it and did they mean to".

**Draw:** a removal with a full history; one with a partial history (applied, but
the actor unknown); one with none at all, which is the genuinely unexplained case
and must stay visibly different from the other two.

---

# PROMPT I — Waiting on a decision, on Today

`/` · Advanced

Today is work, not a summary. It gains a block for merge findings: how many
differences are waiting on a person, and a route to each target holding them.

The rule this block must not break: **findings live per target**, and there is no
combined queue for them. So it links to each registered target rather than
inventing an index that does not exist — a count that leads nowhere is worse than
a count that is missing, because it sends somebody looking for a screen and then
leaves them assuming they misread it.

**Draw:** one target; three targets; and where it sits relative to the blocks
already there (open requests, expiring access, unexplained access).

---

# What I am not asking for

- A restyle of §19–§31. They are built and they are right.
- Light-theme figures. The palette is tokenised and light is derived.
- The member's side of the add-on platform. §20, §26 and §30 cover it.
