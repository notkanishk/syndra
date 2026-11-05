# Q-19 · An extension of you (5 Nov 2026)

Asked by the owner after A-17 and A-18 shipped. Answer: `from-design/A-19-extension-of-you.md`. Boards: `design/a19-*.dc.html`.

## The brief, in the owner's words

"Go through the Syndra design system artifact and Syndra project artifact in depth and improve upon it. Any areas of addition and improvement. Rich, fluid, organic, flowing, elegant user experience. Do your best UI UX. Make the user feel the app is an extension of them."

## What this asks for

Not a new screen. A deep pass over the whole product as built, read against the design system (the Design System artifact, mirrored in `ui/ds-bundle/`) and the Syndra Project canvas, looking for what would make the app feel like it belongs to the person using it:

- **Flow and continuity.** Keeping place across navigation. Context that carries: a person you were looking at follows you into a dialog or the next page. Fewer dead ends, reloads and modal stacks. Back behaviour.
- **Responsiveness.** What happens in the first 100ms after a click. Optimistic feedback. Skeleton versus stand-in. Latency hiding.
- **Motion as a signature.** Breathe, arrive, glide and flash as one language, including shared-element continuity between list and detail where it earns its place. Reduced motion stays first-class.
- **Getting around.** Keyboard-first use, a jump surface, remembered filters and views, anticipating the next step.
- **Organic visual rhythm.** Density, spacing rhythm, layering and depth, light and dark parity, phone width.
- **Design-system gaps.** Missing parts or states, inconsistent parts, tokens that should exist, thin docs and previews.
- Anything else, consistent with A-18's voice.

## Locked facts (what code knows)

- Every holder or access number is observed from Zitadel; records only explain (`internal/claims`, `HolderFacts`).
- Structure never moves in response to data (`basic-advanced-ia/design.md`).
- Every list of names is alphabetical (`ui/src/lib/sort.ts`).
- One write place per setting.
- Plain-language copy: no em dashes, no exclamation marks (`plain-language.test.ts`). The keeper's voice is never used in confirms, errors or logs (copy guide §2).
- Tokens only, never a hex; both themes in full (`ui/src/app/globals.css`).
- Motion: six movements on one curve, `--travel-cap` 10px, breathe the only loop (A-05, A-08, FINAL §4). Tokens: `--ease-tint/press/settle/arrive/flash/breathe`, `--duration-glide`, `--arrive-step`.
- Basic and Advanced views follow the navigation contract in `basic-advanced-ia/design.md`.
- Calm over flashy: nothing that grates on the 50th view.

## What the answer must contain

Findings first, then proposals ranked by impact, each with what, where, why and a cost (S/M/L). A recommended first wave. Numbered owner questions where "yes" accepts the recommendation. New design-system parts as `design/proposed/<Name>.md`. One to three boards in the a16/a18 format showing the key proposals in both themes and at phone width.

Read-only on both artifacts: nothing is published, edited or commented from this answer.
