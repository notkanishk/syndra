# Q-20 · The second pass (5 Nov 2026)

Asked by the owner after A-19's three waves, the first-sign-in welcome, quiet hours, the Pending role filter and the self-hosted fonts shipped. Answer: `from-design/A-20-second-pass.md`. Boards: `design/a20-*.dc.html`.

## The brief, in the owner's words

"Go over everything again."

The standing aim is A-19's: rich, fluid, organic, flowing, elegant; the app should feel like an extension of the user; the best UI and UX.

## What this asks for

A fresh, in-depth pass over the whole product and both design artifacts, read against the code as it now stands (current main, everything through A-19 waves 1 to 3):

- **What A-18 and A-19 left rough**, or what their builds got wrong against their own words. The built code read against A-19 §P1 to P14 and A-18 §2 to §4.
- **New coherence gaps** that the recent waves opened: the motion module, keys, Go to, the forecast tense, remembered views, Back keeps place, the welcome, quiet hours.
- **Areas never deeply designed**: TrueNAS and storage, Insights, Zitadel and the System group, service accounts, the claims page after A-16, error and offline states, empty states across the queues, the phone shell, Basic versus Advanced parity.
- **Design-system gaps**: the nine cards still without previews (ReadGate, ArriveList, GlideIndicator, LeavingRow, FormPart, SectionPill, RoleWordsTable, SignInRow, GoToPanel), and any token or part inconsistency.
- **Anything else** that would make it feel more alive and personal, within the keeper's voice.

## Locked facts (what code knows)

- Every holder or access number is observed from Zitadel; records only explain (`internal/claims`, `HolderFacts`).
- Structure never moves in response to data (`basic-advanced-ia/design.md`).
- Every list of names is alphabetical (`ui/src/lib/sort.ts`).
- One write place per setting.
- Plain-language copy: no em dashes, no exclamation marks (`plain-language.test.ts`). The keeper's voice is never used in confirms, errors or logs (copy guide §2).
- Tokens only, never a hex; both themes in full (`ui/src/app/globals.css`).
- Calm over flashy; nothing that grates on the 50th view.
- No route transitions, no toasts, no letter shortcuts for verbs.
- The Basic and Advanced views follow the navigation contract in `basic-advanced-ia/design.md`.

## What the answer must contain

Findings first, each with a `file:line`. Then proposals ranked by impact, each with what, where, why and a cost (S under a day, M a few days, L a week or more). A recommended first wave. Numbered owner questions where "yes" accepts the recommendation. New design-system parts as `design/proposed/<Name>.md`. One to three boards in the a18/a19 format, loading the design system from `ds/syndra/`, showing the key proposals in both themes and at phone width.

Read-only on both artifacts: nothing is published, edited or commented from this answer.
