# Tasks

Ordering is a dependency chain, not a preference. Anything in Foundations
blocks what follows it; the two marked independent can run at any time.

## 1. Foundations

- [x] 1.1 **Modal is bounded and scrolls.** The panel was `overflow-hidden` with
  no height bound inside a scrim that does not scroll, which does not shrink a
  tall dialog — it clips it, and what it clips is the footer. A long rehearsal
  plan on a short window ended with its confirm button unreachable and no
  scrollbar to admit anything had been cut. `dvh` not `vh`, because a mobile URL
  bar makes those different numbers and the one that lies hides the button.
  `ModalFooter` is sticky to the panel's own bottom edge and opaque; sticky is
  inert on a dialog short enough not to scroll, so the common case is unmoved.
  Inherited by all 28 call sites. `b9e80b8`
- [x] 1.2 **The harness could not observe a media query or a preference.**
  jsdom implements no `matchMedia`, and all three production callers write
  `window.matchMedia?.(…)`, so every run took the no-preference branch and the
  other branch had never executed. Worse: `localStorage` was Node 22's
  experimental global started without a path, with no working `setItem`, copied
  by vitest onto the jsdom window so it shadowed the functional one — meaning
  nothing that remembers a choice could be tested. Both replaced. The first test
  this makes possible is the chime's reduced-motion guard, which had shipped and
  never once run. `6bb82b2`
- [x] 1.3 **Breakpoints named for devices** — `--breakpoint-tablet: 45rem`,
  `--breakpoint-desktop: 67.5rem` — with a canary, because deleting them
  silently returns every responsive class to Tailwind's stock scale while the
  class names keep compiling. `4e9ee9f`
- [x] 1.4 Record the board-versus-rules ruling where a builder will meet it:
  `BOARD-SPEC.md` carries the extracted values, design.md §2 carries the ruling.

## 2. Confirmation, before any removal — *independent of layout*

- [x] 2.1 **The vocabulary**: six words, five outcome kinds, one module.
  Absorbed two private duplicates on the way in — `PlanReview` had declared the
  six effects itself, so a plan and the row that ran it could describe one
  result in two vocabularies; `ErrorState` had its own copy of the failure
  prose, so a read that failed and a write that failed described the same 403 in
  two files. Tests fix the two properties that matter: queued never wears the
  tone of applied, and only refused and failed carry "Nothing was changed."
  `ba95573`
- [x] 2.2 **The drain reports where it ran**, and `lib/toast.ts` is deleted
  rather than migrated — `toastSuccess` and `toastError` had zero callers.
  `outcomeFromDrain` maps warning to `queued` rather than `failed` on purpose:
  "resume again" is not a failure, and an operator who reads it as one goes
  looking for a broken machine. `bf1a91c`
- [x] 2.3 Migrate the remaining toast sites — 83 across 20 files. ~40 are
  `toast.error` and are the only failure signal those screens have, so each one
  is replaced rather than dropped.
- [x] 2.4 Nine test files mock `sonner` and several assert on the mock; they
  assert on the rendered outcome instead.
- [x] 2.5 Remove `sonner` from `package.json` and the `Toaster` from
  `providers.tsx`. A canary fails if either returns.

## 3. The shell

- [x] 3.1 `AppShell` at the breakpoint: `h-screen` → `h-dvh`, the flex flip, and
  the safe-area insets. **`#app-scroll` stays the scroll container** —
  `ui-view.tsx` scrolls it by id for the Basic→Advanced reveal, and moving
  scrolling to the body makes that a silent no-op rather than an error.
- [x] 3.2 The tab bar, built from `navFor(audience)` unchanged. Member 3, Basic
  4 with the Access group landing on Projects and carrying Roles and Apps as a
  header segment. No "More" tab — a fifth slot turns the rail's rule into "four
  things and a drawer of leftovers".
- [x] 3.3 The nav sheet for Advanced, with the Go-to bar. One dot in the highest
  tone present and a count of **destinations** wanting attention, never a
  rolled-up item total: three findings plus eleven expiries plus three holds is
  seventeen of nothing.
- [x] 3.4 `Sidebar` mount decision. If it renders hidden the badge polls still
  run; if it unmounts, `useFlashOnChange`'s `settled` guard resets and every
  badge flashes on each sheet open — which is the exact failure that guard was
  written to prevent.
- [x] 3.5 `TopBar` triage: breadcrumb, view pill (44px, not the drawn 34), the
  account sheet, and sign-out — which is a POST form and can move but cannot
  become a link.
- [x] 3.6 Bottom-anchored collisions: `SelectionBar` (`sticky bottom-4 z-20`)
  and the converge dock (`fixed bottom-6 z-[60]`, above the Modal scrim) need
  the tab bar's height as an offset and a reconciled z-index.

## 4. Sheets

- [x] 4.1 `Modal` renders as a sheet below tablet: 420 short, 520 stopping 96px
  short of the top, 760 full-height with a sticky footer. Reuses
  `useDialogFocusTrap` unchanged.
- [ ] 4.2 Push, never stack. The second sheet replaces the first's content and
  the first's title becomes a back-line; back walks up one level inside the
  sheet before closing it.
- [x] 4.3 A busy sheet cannot be dismissed and says so — silently ignoring a
  dismissal reads as a frozen app.
- [x] 4.4 Keyboard: the sheet's height is `min(content, space above keyboard)`
  and the footer pins to the sheet's own bottom edge, never the viewport.

## 5. Rows, and the touch primitives

- [x] 5.1 `CardRow` gains an optional disclosure; `CardColumns` becomes
  hideable centrally. Additive — every existing caller compiles unchanged.
- [x] 5.2 The 199 fixed-px cells across ~42 files, by density. Watch
  `ListStates`'s `contents` wrapper: a per-row wrapper element changes what the
  flex parent sees and breaks the `arrive-list` stagger.
- [x] 5.3 The copy row learns it cannot copy **before** it is tapped, and
  carries `Select` instead. `navigator.clipboard` is undefined on http, which is
  how this LAN deployment is reached — so the affordance that fails silently is
  the one members meet.
- [ ] 5.4 Selection mode: a named `Select` control on all five surfaces, 24px
  glyphs in 44px rows, select-all that states both numbers, and a count bar
  naming the next step — "Rehearse removal for 9 people", never "Remove 9".
- [x] 5.5 The ladder with a keyboard up. `ConfirmByTyping` gains
  `autoCapitalize="none"` and an `inputMode`; mobile autocapitalisation is
  exactly the failure `typedMatches` was already made case-insensitive to
  forgive.
- [x] 5.6 The freshness strip as one component, **five flags** — `truncated` is
  orthogonal — and the block/allow rule kept unsplit.
- [x] 5.7 The five `title` attributes move into row disclosures. The Zitadel-read
  one is the important one: it is the only explanation for a failed read.

## 6. Screens

- [ ] 6.1–6.18 The Block B screens, per the design reply. Each carries its own
  copy verbatim; the copy is normative.
- [ ] 6.19 The dormant sweep states that the size is unknown rather than
  implying zero — see design §7.

## 7. Platform

- [x] 7.1 Offline, distinct from `degraded`. No client-side queue: a queue in
  the browser is a second, invisible ledger.
- [x] 7.2 Session return, and expiry mid-apply that loses neither the fact that
  nothing changed nor the plan.
- [x] 7.3 Installable: icon, maskable variant, splash that does not animate, and
  the prompt in the Account sheet rather than a first-load banner.
- [x] 7.4 Landscape. Rung 3 is refused with a stated reason rather than squeezed
  — the consequence sentence is the most protected element in the ladder.

## 8. Verification

- [x] 8.1 Browser pass at 390 / 744 / 1280 for every route touched.
- [ ] 8.2 The per-route a11y checklist `BIA-36` and `ISC-43` already owe, now
  including touch targets and dynamic type.
