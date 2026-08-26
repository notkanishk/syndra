# What to hand back

Paste last, once the screens are drawn.

## How to hand it over

**Publish each commission as a design canvas and give me its link.** One canvas
per commission — the target page, then the supporting screens — with the figures
laid out on it. A link I can open is the deliverable; do not describe a canvas
you have not published.

If you are not able to publish a canvas, say so plainly in one line and answer in
prose instead. Prose is genuinely useful and gets recorded verbatim — but I need
to know which of the two I am reading, because a described figure and a drawn one
are not the same thing and I have previously been handed the first while
believing it was the second.

Either way, **finish with a single list of every figure you actually drew**, by
id. Not the ones you discussed. The ones that exist on a canvas.

## Give me

1. **The canvases** — one per commission, published and linked, with every figure captioned.
   The caption is the part I read when the drawing does not cover my case, so it
   carries the *reasoning*, not a label. "Grouped under Health because a target
   that cannot be reached makes the next three panels unanswerable" is a caption.
   "Health card" is not.
2. **A screen-to-endpoint map** — every figure against the endpoint it reads, in
   one table.
3. **What you added to the design system**, if anything: a new component, a new
   tone, a new spacing step. Name it, say what it is for, and say which existing
   thing you considered first and why it did not fit.
4. **What you changed your mind about** between the brief and the drawing, and
   what the brief got wrong.
5. **What you could not resolve** — the questions the brief did not answer and
   you had to guess at. List them as questions, not as decisions.

## Do not give me

**A build plan, a task list, a phased rollout, or a handoff document that tells
me what the codebase should do.**

Not because it would be unwelcome — because it would be wrong, and wrongly
authoritative. You have not read this repository. The last commission in this
bundle produced a handoff saying a particular screen was canonical and that the
existing ones should migrate onto it. A component already existed that did the
same job better, was already used by five callers, and had a refusal-driven
confirmation step that was a better design than the one drawn. Following that
handoff would have meant deleting working, better code. The correction had to be
written afterwards, by reading the repo, into a file that now opens with *"this
overrides the design README where they disagree"*.

So: no statements of the form "X is canonical", "replace Y with Z", or "build A
before B". Where a drawing implies a component should exist, **draw it and say
what it does** — I will find out whether it already exists.

## One thing to be explicit about

This is a **redesign of a page that is deployed and in use**. Its panels are
individually correct and their copy was written carefully; several sentences are
load-bearing and are quoted in the brief.

So for each panel in `REFERENCE-current-page.md`, tell me plainly which of these
your design does to it:

- **keeps it as it is**, moved or regrouped;
- **changes it**, and what changes and why;
- **removes it**, and where its content goes instead.

A panel you do not mention, I will assume you kept. Say so if that is wrong.
