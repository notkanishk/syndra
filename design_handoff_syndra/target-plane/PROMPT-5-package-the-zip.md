# Package everything as a zip

Paste this when the drawing is done. It replaces the "how to hand it over"
section of the previous prompt.

---

Package everything from this conversation as a **single downloadable zip** called
`syndra-target-plane.zip`.

I am going to unzip it directly on top of an existing folder in my repository, so
the layout has to be exact. Nothing above the paths listed below, and no
top-level wrapper directory.

```
design/<Board name>.dc.html      one per commission — see below
design/support.js                the canvas runtime
CLAUDE-DESIGN-REPLY.md           everything you told me, in prose
FIGURES.md                       the inventory
```

## `design/*.dc.html`

**Every view you drew, without exception** — including the ones you drew and then
moved on from. I would rather delete a figure than discover later that one
existed and was never handed over. If a view only exists as a description and was
never actually drawn, it belongs in the prose file, not here.

Two boards is the shape I expect — one for the target page, one for the
supporting screens — but split them however the work actually fell.

Requirements:

- Each board must **open standalone from `file://`**. `support.js` sits beside
  the boards, at `design/support.js`, and is referenced relatively (`./support.js`)
  — if it is missing or the path is absolute, the board renders as unstyled
  markup and the handover is worthless.
- Google Fonts by URL is fine; nothing else external.
- **Every figure keeps its `<figcaption>`**, and the caption carries the
  reasoning rather than a label. The captions are the part I read when a drawing
  does not cover my case.
- Give every figure a stable id in its caption (`T3`, `S1b`) so the prose and the
  inventory can refer to it.

## `CLAUDE-DESIGN-REPLY.md`

Everything you told me in this conversation that is not on a board, in full and
in your own words. Do not summarise it — I am going to record it verbatim as the
design authority, and a compressed version is one I cannot check against later.

Include, clearly separated:

- what you decided about the target page's structure, and the argument for it;
- what you changed your mind about between the brief and the drawing;
- what the brief got wrong;
- what you could **not** resolve — as open questions, not as decisions;
- anything you added to the design system: name it, say what it is for, and say
  which existing component you considered first and why it did not fit.

And the table I asked for: for **each of the eleven panels** in
`REFERENCE-current-page.md`, one row saying **kept / changed / removed**, where it
now lives, and one line of why. A panel you do not list, I will assume you kept —
say so if that is wrong. This page is deployed and in use, so a panel that
disappears silently is a feature somebody loses without anyone deciding to remove
it.

## `FIGURES.md`

A flat table, one row per figure that **actually exists on a board**:

| Figure id | Board file | What it shows | Endpoint it reads | Which of the eleven panels it covers |

Not the figures you discussed. The ones a reader will find when they open the
file. I will check the zip against this table, and a mismatch is the first thing
I will ask you about.

## Still not wanted

No build plan, no task list, no phased rollout, and no statement of the form "X
is canonical" or "replace Y with Z". You have not read the repository; that
reconciliation is mine to write. Where a drawing implies a component should
exist, draw it and say what it does.

## If you cannot produce a zip

Say so in one line, then output each file above as a separate fenced code block
with its full path as the heading, in the order listed. I will save them by hand.
Do not silently substitute prose for a board that was never drawn.
