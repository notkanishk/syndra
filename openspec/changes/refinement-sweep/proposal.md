# Refinement sweep (A-17)

## Why

The owner asked on 4 Nov for "a refinement sweep for consistent, cohesive, elegant, enriching UX and UI". Design read every operator and member screen against RULES.md, FINAL.md and the copy guide, and filed about 300 findings in `design_handoff_syndra/reconcile-2026-10/from-design/A-17-refinement-sweep.md`. Most were the same handful of patterns copied by hand: text styled as buttons, fold chevrons, empty states, dots, dialog footers, page gaps, em dashes, raw ids.

## What changes

- **Rules.** RULES.md gains G1–G19 (buttons, going vs doing, showing more, dialog footer, page rhythm, eyebrows, notices, dots, empty, ids, em dash, said once, no alpha text, names link, one choice card, seats, silence for normal, FilterBar, boxes).
- **Shared parts carry the rules.** One `FoldChevron`; `EmptyState`'s action is an ArrowLink or an outline sm Button; `CardHeaderLink` is an ArrowLink; `accentSoft` and `link-tint` retired; `ActionOutcome` requires its word; `ModalFooter` places a ghost Cancel by one constant; `PageHeader` owns header→content; `FilterBar` holds the read line; `ChosenChip`.
- **Every screen** in A-17 §2 is brought onto those parts, except the claims card (rebuilt to A-16) and the seven owner questions.
- **Guards** in `plain-language.test.ts` and `one-control-surface.test.ts` check the rules a machine can: no em dash in copy, no text button, no lone ghost, no hand-copied fold chevron, no alpha text, no Cancel outside the footer's slot, no raw actor id where a resolver exists, no read line placed by a negative margin, no nav-section eyebrow.

## Owner questions (A-17 §3), parked behind one line each

Q1 footer order (`CANCEL_FIRST`), Q2 Pending's empty Send label, Q3 My access's "rooms" line, Q4 mythology lines, Q5 header→content (`HEADER_TO_CONTENT`), Q6 ids in Advanced meta, Q7 "Grant direct access" (`GIVE_ROLE_LABEL`). See `tasks.md` §Q.

## Impact

UI only (`ui/src`). No API, no migration.
