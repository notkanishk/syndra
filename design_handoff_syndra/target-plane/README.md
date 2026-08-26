# Commission 2 — the target plane

Self-contained. Nothing here depends on the other folders in this bundle.

## What to give Claude Design

Paste these three files, **in this order, in one conversation**:

| # | File | What it is |
|---|---|---|
| 1 | `PROMPT-1-design-system.md` | The design system. Paste it first, on its own, and wait for the acknowledgement. Every later prompt assumes it is in context. |
| 2 | `PROMPT-2-target-page.md` | **The commission.** Redesign `/system/targets/{target}` as one coherent screen. |
| 3 | `PROMPT-3-supporting-screens.md` | Six adjacent screens that were built and never drawn. Paste after 2 — they inherit its structural answer. |
| 4 | `PROMPT-4-what-to-return.md` | What to hand back, and what not to. |
| 5 | `PROMPT-5-package-the-zip.md` | Ask for it as a zip laid out to drop straight into this folder. Paste last. |

`REFERENCE-current-page.md` is **not** a prompt. It is the panel-by-panel
inventory of what the page holds today, with each panel's endpoint and data.
Attach it alongside prompt 2, or paste it after prompt 2 if the tool takes only
text.

If the conversation is restarted, paste `PROMPT-1` again.

## Why this exists

`/system/targets/{target}` **was** designed — twice, and both times as *four
questions about one add-on*:

- `../design/Syndra IA.dc.html` **§21**, four stacked sections.
- `../mobile/Syndra Mobile.dc.html` **M20**, the same four as horizontal tabs.

It carries **eleven panels** now. Four of them are the ones that were drawn; the
other seven arrived one at a time, each defensible alone, and the four-question
spine is no longer visible in the result.

So the page's *panels* were designed and its *composition* never was. That is
what prompt 2 commissions, and it is a redesign rather than a restyle.

Six further surfaces appear on no board at all — the add-on index, merge
findings and their decision form, the target's own system health, the managed
roster, the two findings that render inside Health, and the applied history on a
grant somebody removed by hand. Prompt 3.

## Nothing to download?

Ask for a zip — `PROMPT-5`. It specifies a layout that unzips straight on top of
this folder, so the boards land in `design/` with their runtime beside them and
nothing has to be moved afterwards.

Claude Design otherwise answers in the conversation and there is often no bundle;
that is what happened on the last commission. `HOW-TO-LAND-THE-REPLY.md` covers
the fallbacks — paste the prose into a Claude Code session and have it filed, or
hand over a canvas URL if one was published.

## What comes back — and what to do with it

**Canvases and captions. Not a build plan.** Prompt 4 says so explicitly, and the
reason is in this bundle's own history: the first commission's handoff declared a
screen canonical and told the developer to migrate the existing ones onto it. A
better component already existed, with five callers. `../BUILD-NOTES.md` §2 is
the correction, and it opens by saying it overrides the design README — because
the README was written before the code was visible.

None of that was Claude Design's fault. It had not read the repository and could
not have. The lesson is only that **the reconciliation is the reader's job, not
the designer's**, and that it is written afterwards.

So the sequence is:

1. Drop the canvases in `design/` beside this README.
2. Read them against the code and write the reconciliation — panel by panel: what
   the redesign changes, what is already right, and what it got wrong about the
   codebase. `../BUILD-NOTES.md` is the shape going in;
   `openspec/changes/one-control-surface/BOARD-AUDIT.md` is the shape coming back
   out, recording what shipped against what was drawn.
3. Only then break it into work.
