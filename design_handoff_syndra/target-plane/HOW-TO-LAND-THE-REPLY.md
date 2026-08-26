# Getting the reply out of Claude Design and into the repo

Claude Design answers **in the conversation**. There is usually nothing to
download, and that is normal — it happened on the last commission too. From
`../mobile/CLAUDE-DESIGN-REPLY.md`:

> The reply arrived as a message, not as a bundle.

Only part of that pack was ever drawn (`A1` and `A2`); everything else was
answered in prose. The prose was the deliverable, and it was landed by being
recorded here verbatim.

So there are three routes, and the first one always works.

## 1 · The prose reply — always available

Copy the whole reply out of the conversation and put it in
`CLAUDE-DESIGN-REPLY.md` beside this file, **verbatim, below a provenance
header**. Do not summarise it and do not tidy it. The header records:

- the date it arrived;
- which figures were actually **drawn** versus answered in prose;
- any claim in the reply that is already stale against the code, with the file
  and line that disproves it.

That header is the whole point. Last time the reply asserted two endpoints were
missing that already existed, and the correction is what makes the file safe to
read a month later.

Simplest mechanic: **paste the reply straight into a Claude Code session and ask
for it to be filed.** It writes the file, reads it against the repo, and produces
the provenance header in the same pass.

## 2 · A published canvas — if there is one

If Claude Design published a canvas, it has a link. Give Claude Code **the URL**;
it can read a published artifact directly and save the page into `design/` beside
this file. No download, no export, no copy-paste.

`PROMPT-4` now asks for this explicitly, because the pack previously said "draw"
without ever saying "publish it and give me the link" — which is most of why
there was nothing to hand over.

## 3 · Saving the page by hand — last resort

If a canvas exists and cannot be linked, save it from the browser as HTML into
`design/`, keeping any `support.js` beside it. The canvases in `../design/` and
`../mobile/` are that format: one `.dc.html` per board, plus one shared runtime
file that must sit in the same folder or the board renders as unstyled markup.

## Then, and only then

Read what arrived against the code and write the reconciliation. `../BUILD-NOTES.md`
is the shape. That step is not optional and it is not Claude Design's job — see
`README.md`, "What comes back".
