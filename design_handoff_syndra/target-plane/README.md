# Commission 2 — the target plane

Self-contained. Nothing here depends on the other folders in this bundle.

## What to give Claude Design

Paste these three files, **in this order, in one conversation**:

| # | File | What it is |
|---|---|---|
| 1 | `PROMPT-1-design-system.md` | The design system. Paste it first, on its own, and wait for the acknowledgement. Every later prompt assumes it is in context. |
| 2 | `PROMPT-2-target-page.md` | **The commission.** Redesign `/system/targets/{target}` as one coherent screen. |
| 3 | `PROMPT-3-supporting-screens.md` | Six adjacent screens that were built and never drawn. Paste after 2 — they inherit its structural answer. |

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

## What comes back

Design canvases (`.dc.html`). Drop them in `design/` beside this README, and
record what shipped against them the way `BOARD-AUDIT.md` does for §19–§31.
