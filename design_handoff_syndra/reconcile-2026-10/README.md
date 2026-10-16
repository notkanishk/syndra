# Handoff: Syndra October reconcile

## Overview
The reconcile between the design boards and the shipped app (`ui/src` at `c940f93`). Every row of the earlier consensus ledger (#1–#111) and 23 new differences (N1–N23) has a verdict, settled by the owner by 14 Oct 2026. Nothing is open. This package is what to build.

## About the design files
The `design/*.dc.html` files are **design references made in HTML**, not production code. They render the real Syndra DS components (`ui/ds-bundle`) and the real shell at true width, so they show the intended look, copy and behaviour exactly. Recreate them in `ui/src` with its existing components, hooks and patterns. Do not copy markup from them.

To open a board: serve this folder (`npx serve design` or any static server) and open a board in a browser. Each shows its frames side by side; `?frame=<name>` opens one frame alone. They load the DS from `design/_ds/`.

## Fidelity
High fidelity. Copy is final and verbatim: use the exact strings on the boards and in BUILD-LIST.md. Colour, type, spacing and radii are the DS tokens (`--accent`, `--warn`, `--surface-1`, `--radius-panel`…); no literal colours appear anywhere, and none should be introduced.

## Read in this order
1. **RULES.md**: the standing rules every board follows (Notices, vertical rhythm, links vs buttons, sorting, folding lists, "Waiting for you to send →", freshness dots, phone behaviour). Build to them wherever a board is silent.
2. **BUILD-LIST.md**: what to change, by screen. Backend changes first (B1–B11); frontend items refer back to them.
3. **00-CONSENSUS.md**: the ledger, each row with its verdict and the boards that draw it, plus the New differences table.
4. **verdicts.md**: the reasoning, one paragraph per verdict.
5. **proposed/**: components to add to the DS once and use everywhere (Notice, NextStep, SelectionSeat, DegradedBanner, ActionOutcome seat, StatCard href, VersionBand, ConnectionGuide, Expiry sweep run, Deliberate later), plus implementation notes for Connected systems (Mapping-implementation-notes.md) and Zitadel (Zitadel-code-notes.md).

## Kinds of verdict
- **KEEP-CODE**: as built; the board was updated to match.
- **SYNTHESIS**: the drawn frame is the target. A letter (32a, 87c) names the owner's pick.
- **Carried**: already Keep, Locked or Owner-ruled; every board follows it.
- **SUPERSEDED**: replaced by the verdict named beside it.

## Boards (design/)
- Shell and nav: shell-account, Sidebar, route-home
- People: route-users, route-users-id
- Access model: route-projects, route-projects-id, route-roles, route-applications, route-applications-id, route-bundles-v2
- Requests and review: route-requests, route-drift, route-expiring, route-holds, route-revocations, route-pending
- Automation and changes: route-automation-settings-v5 (Roles that wait; v2 kept for option A of N14 in full, not built), route-insights
- System: route-systems, route-system-targets-id, route-system-targets-id-mappings, route-zitadel, route-events
- Member: route-me, route-storage
- Across the product: route-mobile, route-login, route-states, Motion (long-list motion 8a–8d), Motion v2 (A-05), Cohesion, Controls (A-04)
- Asking and undoing (A-04): route-asking, flow-cancel-queued
- New proposed components (A-04): AnchoredPanel, ChoiceRow (with Checkbox and Radio), CancelQueuedStep; ActionOutcome's block form gains a word

## Suggested build order
Each step ships on its own.
1. **Foundations:** Notice (banners, errors, offline in the content column), ActionOutcome queued copy, "Close" for dialogs with a result, SelectionSeat, button grammar, "people" never "holders". BUILD-LIST: States 1–4, Selection, Buttons, Roles 1.
2. **Never sample data (N23):** B11 and States 5.
3. **Confirmation, option B:** the `waits_for_send` role flag, WaitsMark, RolePicker, Next step, Roles that wait, nav 3a.
4. **Screens:** Bundles v2 (B5–B9), Insights, My access (B10), Storage (B1–B2), Connected systems (B3), People bulk (B4).
5. **Phone and login:** Mobile 1–5, Login 1 and 3.

## Interactions, states, motion
- Every list has four states (loading, empty, error, degraded) drawn on route-states; loading and error frames carry no counts, notes or freshness.
- Long-list motion (fold, show more, back to 10, re-sort, jump, floating pill) is specified on Motion with durations and easing; nothing moves under the reader, and all of it respects reduced motion.
- Phone (below tablet): Filters opens as a Modal sm sheet ending in one accent button that names the result; Insights becomes sentences (route-mobile).

## Not in this package
- **Benched:** route-additions (role sheet, Jump to anything, send one batch, daylight; N24–N27). The owner will return to it.
- **History:** the August boards and superseded boards (route-bundles v1, route-graph v1/v2, automation settings v1, v3, v4) stay in the design project. Where the August boards say degraded mode shows demo data, N23 overrides them.

## Assets
None beyond the DS bundle and fonts in `design/_ds/`. People, roles and numbers on the boards are fixtures.
