# Design notes — violet, healthy, and motion

Decisions that were arguable, and why they went the way they did. Everything
here was reached against `design_handoff_syndra/`; where this deviates from the
handoff it says so.

## 1. The accent swap is a token edit; the healthy split is not

Every accent usage in the app already went through `--accent`/`--accent-text`/
`--accent-ink`, so pointing those at the violet ramp moved 110 usages across 50
files without touching a component. That was the easy half and it is worth
naming as the reason the design system existed in the first place.

The half that could not be mechanical: lime does not disappear, it changes
meaning. Under the handoff violet **never** means "good" or "safe", so every
place the old accent was standing in for "this is fine" had to be found and
moved to `--healthy` by hand — because a find-and-replace would have made those
places say the opposite of what they mean.

## 2. `--accent-dense` exists because of a contrast failure, not a preference

`--accent-ink` (`#f7f4ff`) on `--accent` (`#7f5af0`) is 4.18:1. That passes AA
as large text — ≥18.66px bold or ≥24px — and fails for anything smaller. Every
filled control in this product is a 13–13.5px semibold pill, and the rail's
count badges are 11.5px. All of them were failing.

`--violet-600` (`#6f4ae0`) takes the same label to 5.2:1. Rather than darkening
the brand fill everywhere — which would have cost the doorway its brighter
button, where the label is large enough to be fine — the split is by what the
fill carries:

> **`--accent` for fills that carry no text. `--accent-dense` for fills that do.**

Dots, bars, checkboxes and selection indicators keep the brighter violet;
buttons, badges and segmented controls take the denser one. A `design-system`
test fails on any `bg-accent` in the same class string as `text-accent-ink`, so
the rule cannot quietly rot.

## 3. The healthy token has no siblings, and that is the enforcement

Every other semantic role has four tokens: `--warn`, `--warn-text`,
`--warn-ink`, `--warn-soft`, `--warn-line`. `--healthy` has one.

The handoff says lime is "a dot, a word, or a hairline — never a button and
never a fill behind text". The cheapest way to hold that is not a review
convention but the absence of the tokens a fill would need. There is no
`--healthy-ink`, so there is nothing to put a label on a lime field *with*. A
test asserts the siblings stay absent.

## 4. Healthy is a dot, and the first attempt got this wrong

The first pass tinted the Today health cells' values lime — four 26px lime
numerals reading "Reachable / 0 / 0 / 0". Every one was defensible against the
letter of the handoff ("a word") and the result was that "nothing is wrong"
became the loudest thing on the page.

That is the opposite of what the handoff asks for: *"the healthy state, which
earns its calm by being the only thing on screen holding perfectly still"*. The
values went back to `--ink`, and healthy is now a 6px dot beside the note. Same
information, correct volume. The board's own health specimen is a dot and a
small word, never a large value — that is what "a dot, a word, or a hairline"
means in practice.

## 5. Empty means two different things, and only the caller knows which

An empty triage queue is *resolved*: work existed, and none of it needs you. An
empty people list is merely *absent*: nobody has been added, which is not good
news, it is just news. Only the first earns a healthy dot.

`EmptyState` cannot tell them apart from a zero, so `resolved` is an explicit
prop and defaults to false. Five of the app's twenty-eight empty states set it:
expiring access, pending changes, the pending requests tab, unexplained access,
and reconciliation. "Nobody holds this role yet" pointedly does not — the
handoff treats that as a problem, not a clear queue.

## 6. Motion is spent through named roles, never raw durations

`motion-tint`, `motion-press`, `motion-settle` bundle a duration with its
easing, so the two cannot drift apart, and the class name is the argument for
the timing. A row that starts lifting on hover now reads as wrong in the diff
rather than only in the browser. A test bans `transition-colors`,
`transition-all` and raw `duration-*` from source.

## 7. `arrive` is CSS and lives at the choke point

The stagger could have been an index prop threaded through every list. Instead
`ListStates` — the wrapper every list in the product already passes through so
it cannot skip an empty state — wraps its children in `.arrive-list`, and the
stagger is `:nth-child`.

Two things fall out. The cap cannot be got wrong at a callsite: rows 1–6 get
0/40/80/120/160/200ms and everything after is pinned at 200ms, so a queue of
forty resolves in the same 620ms as a queue of six. And a new list gets the
behaviour by *being* a list.

The wrapper is `display: contents`, so it adds no box — this sits inside flex
and grid parents everywhere and a real element would collapse the row gaps.

## 8. Reduced motion had to collapse delays, not just durations

The pre-existing reduce block neutralised `animation-duration`,
`animation-iteration-count` and `transition-duration`. With `arrive` staggering
up to 200ms that was no longer enough: the sixth row of every list would sit
invisible for a fifth of a second and then appear. A stall, not an animation —
precisely what the preference asks us not to do. `animation-delay` and
`transition-delay` now collapse too, and a test holds all five.

## 9. `flash` is the one thing that cannot be CSS

"A value changed while you were looking elsewhere" is not knowable from a
stylesheet. `useFlashOnChange` is deliberately small and has two properties
worth stating: it never fires on first paint, and it watches the *value* rather
than the fetch — a poll returning the same twelve unexplained grants is not
news, and must not look like it.

`FLASH_MS` duplicates `--duration-flash`, which is a real (if small) seam: the
constant decides when the class comes off and the stylesheet decides how long
the animation runs. A test asserts the two numbers are equal rather than trying
to derive one from the other at runtime.

## 10. The spinner became a breathing dot

`animate-spin` on a pending button was a third looping animation in a product
that licenses two. The rule's own logic covers the case — a submitting button
*is* "still happening" — so the fix was to say it in the vocabulary that already
exists rather than to add a second idiom for the same statement.

## 11. The mark is one drawing, scaled

`.syndra-mark` carries every colour and both masks; the component is three
spans and a `--mark-size`. The rail (22px) and the favicon are then the same
drawing rather than two that have to be kept in agreement, and the light theme
inverts it without the component knowing.

One trap, stated in the handoff and worth repeating because nothing looks wrong
when you hit it: the base ring's radial fade must keep its opaque stop at **88%
or beyond**. At 62% on a 22px box the mask erases the 1.5px border completely,
because that border occupies 93.5–100% of the mask radius.

## 12. The favicon is SVG, and departs from the CSS mark twice

Both departures are forced by the medium, not chosen:

- The arc's falloff is two solid-stroke bands rather than a conic mask. SVG has
  no conic gradient, and a gradient *stroke* flattens to its first stop when
  rasterised — the fade vanishes and the source still looks correct. Solid
  strokes and gradient *fills* both survive, so the glow is a fill.
- Colours are literal. A favicon is served standalone with no page and no
  custom properties to inherit. The theme it follows is the operating system's,
  which is what the `prefers-color-scheme` block inside the SVG is for.

No PNG fallback ships. SVG covers every desktop browser and Android Chrome; a
raster apple-touch-icon is the only real gap and it is a home-screen icon, not
a tab.

## 13. Three defects found in review, and what they had in common

All three were the same shape: an animation or a signal that was *correct in
isolation* and wrong about context.

**The scrim was moving.** `settle-scrim` pointed at the `arrive` keyframes,
which translate 8px. Two things followed, and the second is worse than the
first. A `fixed inset-0` element translating upward leaves an unpainted strip
along the bottom of the viewport. And because the card is a **child** of the
scrim, the two transforms compounded — the dialog rose 18px, breaking the one
hard number in the motion system, that nothing ever travels more than 10px.
`fade` is now an opacity-only keyframe and a test resolves the scrim's
animation name and asserts its keyframes are transform-free. Measured: scrim
never translates, card travel exactly 10px, no bottom gap.

**Every nonzero badge flashed on page load.** `useIndicators` uses
`placeholderData` so a failed poll can never blank a badge — real safety, and
the cost is that the rail holds four fabricated zeros before the first payload.
Nothing downstream could tell them from real zeros, so the first real `12`
read as a change from `0` and flashed. `useFlashOnChange` now takes `ready`,
and the gate considers the *previous* render as well as the current one,
because the commit where placeholder data is replaced carries a value change
and a readiness change together — and that is an arrival.

**The same bug was already in the drift chime**, and worse: an audible alert
on every page load, in code whose own comment says a chime that fires
routinely "would be trained out within an hour." Same root cause, same fix.

**A second change inside 900ms was never marked.** `setFlashing(true)` while
already true is a no-op in React, so the class never left the element and CSS
would not replay — the timer restarted and nothing else. Measured in Chrome:
re-applying the class without releasing it left the animation at
`currentTime: 508ms`, still running out the first flash. The hook now drops
the class for one frame; the same measurement then reads `currentTime: 8ms`,
a genuinely fresh animation.

### The test that passed for the wrong reason

The original suite had a case called "restarts rather than ending early". It
asserted that the class was still present for a full `FLASH_MS` after the
second change — which was true, and which was exactly the bug. The class was
present because it had never left. A test can confirm a symptom of the
behaviour it is meant to guard and still be blind to it; this one is now
written against the class *clearing*, which is the thing that makes CSS
replay, and it fails when the release is removed.
