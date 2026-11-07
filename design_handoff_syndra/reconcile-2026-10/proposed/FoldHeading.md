# PROPOSED — Fold heading (a group, folded)

Status: built 8 Nov 2026 (A-21 §2.9, architecture deepening 1.7), `ui/src/components/ui/FoldHeading.tsx`. It was `components/bundles/FoldHeading.tsx` (role picker folds, Retired bundles, Retired rules); A-21 asked whether a heading part already existed, and this was it. For the Design System artifact: not yet published there; the canvas draws a plan's folds inline (`a21-plan-grouped`, `a21-plan-result`).

## Anatomy
FoldChevron (28px round) · optional StatusDot (warn, healthy, danger; healthy may bloom once) · the word, 14.5/600 · the count in `--faint`, 13px ("4 people") · optional faint line beneath, 12.5px, indented to the word ("Because Trained in Laser Lab always waits for you.").

## Link form
When the word is a link ("Waiting for you to send →" after a plan's Apply), the word is NextStep with its own amber dot, and the chevron alone is the fold button (44px touch floor, `aria-label` the word), so a link never sits inside a button.

## How a plan uses it
`PlanReview` wraps each fold and its rows in one `role="group"` labelled "{word}, {count}" ("Will wait for you to send, 4 people"); the heading is sticky at the box's top on `--surface-0`; ten rows, then PaginationRow. Open by default.

## Never
A › chevron, a muted text toggle, a tinted box, a colour other than the dot's.

## Not yet
Pending changes' own batch fold rows (`governance/pending/page.tsx`) still draw their heading by hand; they move onto this part next (A-21 §6.7).
