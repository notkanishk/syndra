# Six screens that were built and never drawn

Paste after the target-page brief. Each of these inherits whatever structure you
landed on there, and drawing them first means drawing them twice.

Six screens, A–F. Draw them in order; A is the only one that is a page of its
own, and B is the biggest.

---

## A · Connected systems

`/system/targets` · `GET /api/v1/targets` · Advanced › System › Connected systems

The index of what this deployment has registered. It exists because a deployment
with **no** add-on registered previously showed nothing about add-ons anywhere —
no row, no page, no sentence — and an operator reads that silence as *the
platform did not ship*, not as *it is not configured here*. The empty state is
the reason this screen exists. Draw it first and draw it best.

Per target: its name, its id, one reading of whether it is answering, and how
many operations it offers. The reading is **four states that are not
interchangeable**, because each sends an operator to a different machine:

- registered but no manifest read yet — a runtime fact, not a fault
- transport secret will not load — a fault on *this* host
- breaker suspended — Syndra refusing its own calls
- answering

A target that has published no manifest shows **no operation count at all**. `0`
there is a claim about the target rather than about Syndra never having asked.

**Empty state** says: no system is connected; what an add-on is, in one sentence;
what registers one (`ADDON_TARGETS` in the deployment's environment, then its
container); and what is consequently *not* happening — Syndra is governing
identity only, and nothing outside the identity provider is being reconciled.

**Also answer:** should this screen exist once a target is registered, or should
it be the place per-target rows live rather than a peer of them? The navigation
currently shows both the index and a row per target, which may be one row too
many.

**Draw:** one target answering · one registered and never answered · one with a
broken transport · the empty state · the error state.

---

## B · Waiting on a decision

Target page · `GET /api/v1/targets/{t}/merge-findings` ·
`POST …/merge-findings/{id}/resolve`

Reconciliation is a **three-way merge**. It knows what the target last reported
(*base*), what Syndra wants (*ours*), and what the target holds now (*theirs*).
Most differences resolve themselves. Three do not:

| Outcome | What happened |
| --- | --- |
| `theirs_only` | Syndra did not change it, so somebody changed it on the target |
| `conflict` | both moved, differently |
| `deleted_upstream` | the account is gone |

**Every row must say what it used to be.** That is the question an operator asks
first, it is what no surface could answer before the merge base existed, and a
row without it sends somebody to a target history that for most targets does not
exist.

**The decision** is one of five, each with a mandatory reason: keep Syndra's ·
take the target's · provision it again · stop managing it · record that the two
sides now agree.

Taking the target's value is **only offered when there is a per-subject home for
it**. A group is produced by a role mapping that reaches everyone holding that
role, so "just for her" is not a thing that exists. When it is not offered the row
says why, and names the governing mapping with how many people hold that role.

**A decision is not a settlement.** Deciding queues work; the row closes when a
later pass sees the target agree. A decided row is still standing and must read
as *decided and waiting* — saying "resolved" there is the surface claiming a
difference is over while it is still on the target.

**One repair case:** a "stop managing it" whose release reached the target but
whose follow-up transaction failed leaves a half-done unbind. That row offers to
finish, and says that pressing again changes nothing on the target.

**Draw:** the three outcomes · an adoptable conflict and a non-adoptable group
finding side by side · the decision form open · a decided-and-waiting row · the
repair row · the empty state, which is good news and should look like it.

---

## C · What the target reports

Target page · `GET /api/v1/targets/{t}/system-health`

Directly beneath Health, and the distinction is the point: the card above answers
*is Syndra able to talk to it*, this one answers *and is the machine itself all
right*. Two authorities, two failure modes, never one chip. A failing disk
surfaces here and nowhere else in the product.

Carries alerts, pools, services, and a `degraded` list naming which of those
could not be read.

**Draw:** healthy · one warning alert (`/dev/sde`, uncorrectable error, open
since July — this is the live one) · a pool at 94% · a partial read where pools
answered and alerts did not · the whole thing unreadable.

---

## D · The change record has been edited

Inside Health · `POST …/log-anchor/resolve`

The add-on keeps a hash chain of every mutation. Syndra anchors what it last saw:
a head and a record count. When the target reports **fewer records** than the
anchor, or the **same number hashing to something else**, the chain has been
edited — and a chain verifies its own contents and cannot notice its own
truncation, so this is the only thing that can.

The anchor does not advance past a finding. It stays until a person resolves it,
and resolving means **re-baselining to a cited head**, never dismissing: there is
no state where this is acknowledged and the anchor is still frozen, because that
state detects nothing and reads as handled. The head is cited *in the dialog*
rather than read at the moment of the click — a chain that changed again while
the dialog was open is exactly the event this exists to notice.

The records that went missing stay missing. Say so.

**Draw:** both violation reasons · the resolve dialog with its cited head and
mandatory note · how it sits above the reachability reading.

---

## E · Two records disagree about who owns an account

Inside Health · `POST …/binding-conflicts/{id}/resolve`

A change for one person was applied to an account, and Syndra's own binding says
that account belongs to somebody else. **Both records were written by this system
and neither is authoritative** — which is why it needs a person and not a
reconcile, and why the design must not call either one correct.

It renders above the reachability reading because a target being down is
temporary and this is not: it stands whether or not the add-on is answering.

Resolving moves the account between people **in Syndra's records and touches
nothing on the target**. That is the whole copy problem — an operator who reads it
as "fixed" will not converge afterwards, and a convergence for either person acts
on whichever record it reads.

**Draw:** the finding with both claimants named and neither preferred · the
dialog · what it says about what has and has not changed.

---

## F · The applied history on an unexplained grant

`/governance/drift` · `GET /api/v1/governance/drift/{id}/origin`

A grant that Syndra applied, and that somebody then removed by hand in the
identity provider, is listed as an independent mystery with no history. It is
not: Syndra remembers proposing it, applying it, and the provider accepting it.

The story a row can now tell: who granted it and why · when Syndra applied it ·
who the identity provider says removed it · when it was last seen.

**Lead with applied.** That Syndra did this is what distinguishes it from a grant
nobody can account for, and it changes the operator's question from *where did
this come from* to *who undid it, and did they mean to*.

**Draw:** a removal with a full history · one with a partial history (applied,
but the actor unknown) · one with none at all, which is the genuinely unexplained
case and must stay visibly different from the other two.
