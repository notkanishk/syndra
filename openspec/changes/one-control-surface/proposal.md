# One surface per control

## What is wrong

The add-on screens shipped, and on them the same act was drawn three ways.

On one target page: `Change` was borderless text, `Remove` beside it was a red
outline pill, `Adopt` two cards down was borderless text again, and `Decide` in
the card between them was a bordered pill. Four row actions, three treatments,
no rule distinguishing them — `TargetOverview.tsx` contained both conventions
itself, using `outline` for the confirm step of a release and `ghost` for the
row action that opens it.

That is the visible half. Underneath it, six controls had been rebuilt by hand
rather than used:

| What | Where | How the copy had drifted |
|---|---|---|
| The outline button | drift queue, people list, person activity | `motion-tint` instead of `motion-press press-scale` — it does not press — and a pixel short above `desktop:` |
| The page tab row | person page, drift queue | `min-h-11` in one and `min-h-[44px]` in the other; padding always in one and only above `desktop:` in the other |
| `Badge` | five files | four backgrounds, two paddings, and one branch of `RiskPill` that had lost `font-semibold` — so a role group rendered bold or not depending on whether it was a safety group |
| The status dot + word | target health, add-on index | the index used colour alone, which is no signal at all to a reader who cannot separate the hues |
| The pill metric | seven controls | `px-3.5` or `px-4`; `13px`, `13.5px` or `14.5px`; `py-1.5` or `py-[7px]` |
| An identifier in prose | eight sites | mono at `12.5px`, `13px`, `13.5px`, or whatever the paragraph was |

The last one is why the drift queue's header showed three rows of pills at three
different heights: a filter, a tab and a button, stacked within a hundred pixels
of each other, each carrying its own numbers.

## What changes

**A rule for which variant a control wears**, applied across the target plane:

- The one action a row or finding offers is `outline`. It is the only affordance
  in that row, and a borderless control in a table reads as text until it is
  hovered — which never happens on a touch device or in a screenshot.
- A destructive row action is `danger`, the red outline. Unchanged.
- `ghost` is the quieter half of a pair, and only that: Cancel, Dismiss.
- A dialog's confirm stays `accent` or `dangerConfirm`. Unchanged.

**One definition per surface.** `PILL` is exported from `Button` and imported by
every control that is that box without being that component — the view switch,
the sound switch, a request's duration choice, `FilterPills`, `Tabs`. `Tabs` is
new and replaces the two hand-rolled tab rows. `Badge` gains the two soft tones
it was already being asked for inline. `StatusDot` is shared between the target's
health readings and the add-on index.

**A guard**, because every finding above was found by looking at a screen.
`one-control-surface.test.ts` reads the source and fails on a hand-rolled pill
control or status pill outside `components/ui`. It found five more offenders than
the visual pass did.

## What does not change

- No copy, no information, no route, no behaviour. Only which component draws
  what, and what the numbers are.
- The shell's own chrome — the rail, the tab bar, the account sheet — is exempt
  and stays hand-built. Those are one-off compositions, not instances of a
  control that exists elsewhere. So are the error pages, which must not depend
  on the thing that may have failed.
