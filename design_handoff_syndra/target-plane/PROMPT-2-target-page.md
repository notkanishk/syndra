# Redesign one page: an add-on target's operator screen

`/system/targets/{target}` · Advanced › System › ‹target› · desktop, 1440

This page is built and deployed. **I want it redesigned as one coherent screen**,
not restyled. The panels on it are individually right and the composition is not
a composition — it is eleven stacked cards in the order they were added.

## How it got here

It was drawn twice, and both times as **four questions about one add-on**:

> Is it answering, whose accounts are on it, what can it actually do, and what
> state has somebody put it in. In that order, because each answer changes what
> the next one means — capabilities read differently once you know the circuit
> is open.

Four of the eleven panels are those four questions. The other seven arrived one
at a time over the following months, each defensible on its own, and the spine is
no longer visible. An operator arriving with a specific question — *why can this
person not get in?* — has no route to it and scrolls.

`REFERENCE-current-page.md` lists all eleven with their endpoints and data. Read
it before drawing.

## The problem, stated precisely

Three things happened that the original four questions did not anticipate.

**1 — A second category appeared: things that need a person.** Reconciliation now
compares three states (what the target last reported, what Syndra wants, what the
target holds) and refuses to resolve some differences on its own. Alongside those
sit a change record that has been edited and two of Syndra's own records
disagreeing about who owns an account. None of these is a *status*. Each is a
piece of work waiting on a human, and they are currently scattered: two render
inside the Health card, one is its own panel two-thirds of the way down.

**2 — The page grew a second subject.** Six panels are about the *target* — is it
up, what can it do, what state is it in. Five are about *people and their access*
— who has an account, what roles reach here, who is dormant, who is disputed. One
page, two subjects, no seam.

**3 — "Whose accounts" is now three lists, not one.** The board drew only the
accounts Syndra did *not* create. There are now three populations — managed,
dormant, unmanaged — and they must never be one count, because each one means
something different and each takes a different action.

## What I want you to decide

**Does the four-question spine survive?**

- **If it does** — show the grouping and defend it. Health absorbs the target's
  own hardware readings; "whose accounts" absorbs all three populations; "what it
  can do" absorbs the role mappings and their published versions; "what state is
  it in" absorbs reconciliation and maintenance. And say where the work-waiting-
  on-a-person category lives: a fifth question, or distributed back into the four.
- **If it does not** — say what replaces it, and why the original argument no
  longer holds.

Either way the answer must survive these:

- **Nothing appears or disappears with its count.** A page with no outstanding
  findings and a page where somebody's access is disputed must be *legible* as
  different without the second one having grown a panel the first lacks. This is
  the constraint most likely to break a tabbed design, and it is not negotiable.
- **The failure case is the one that matters.** Draw the page for a target that
  is **not answering**. Nine of the eleven panels have nothing to say, and this
  is exactly when an operator needs the page most. Today it renders as a column
  of empty cards.
- **Two subjects, one page.** Either give people-and-access its own region, or
  argue that splitting it onto a second screen is right — in which case say what
  the target page keeps and what the navigation looks like afterwards.
- **Desktop is not the phone.** The touch board made this four horizontally
  scrolling tabs. If you tab the desktop too, say what happens to a finding that
  lives under a tab nobody is looking at. If you do not, say why the desktop can
  carry what the phone could not.

## One panel I already suspect is misplaced

**What roles reach here** and **Published versions** were designed as a separate
*mapping detail screen* with version history and a blast-radius acknowledgement,
and shipped as two panels on this page instead. Editing one mapping moves access
for everybody holding that role — a fact with no room to breathe in a table row.
Tell me whether that belongs here at all.

## Draw

1. **The page, redesigned**, at 1440, with real content: a healthy target, two
   people bound, two unmanaged accounts, two role mappings, six operations.
2. **The same page with work outstanding** — three merge findings, one edited
   change record. This is the figure that proves the urgency answer.
3. **The same page, target not answering.**
4. **The same page on a fresh deployment** — registered, nothing bound, nothing
   mapped, no history. This is the live state today and it must not read as
   broken.
5. **One 390 × 844 figure** reconciling your structure with the touch board's
   four tabs.

## Content you may treat as fixed

The copy on the existing panels was written carefully and most of it should
survive. Change it where your structure requires it, and say so in the caption.

Two sentences in particular are load-bearing and should not be softened:

- Reconciliation *"reads the target and queues what is already owed. Queueing is
  not applying."*
- The unmanaged inventory is *"reported, never triaged. These are not drift."* —
  a real NAS holds `root`, service accounts and whatever an admin made by hand,
  and rendering those as untraced access would bury the triage queue on the first
  sweep after deployment.

## What I am not asking for

A new design system, a light-theme variant, or a redesign of the member's side of
the add-on platform. Those are done.
