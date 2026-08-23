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
- [x] 4.2 Push, never stack. **Nothing in the product nests a dialog** — the
  whole suite runs without tripping the guard, which is the evidence — so the
  work was making that a property rather than a coincidence: a context says
  whether a dialog is already open above this point, and a second one
  complains in development. It still renders, because refusing would trade a
  layout problem for a missing confirmation. The push-and-back-line shape is
  specified and unbuilt, waiting for the first screen that needs two steps.
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
- [x] 5.4 Selection mode, on all five surfaces plus the dormant sweep, which
  had drawn its own 16px input — the smallest target in the product in front
  of the most destructive action in it. Three things the old shape got wrong
  beyond the glyph size: **select-all lived in the column header**, which does
  not exist below the tablet breakpoint, so phones had no select-all at all;
  **`BULK_MAX_USERS = 500` was exported and never read**, so a selection over
  the ceiling met either a silent trim or a dead bar; and **nine bar verbs
  named outcomes their taps do not produce** — every one of them opens a
  rehearsal. Deviation from A4: the count is stated once, in the bar's
  sentence, rather than repeated inside five sibling buttons.
- [x] 5.5 The ladder with a keyboard up. `ConfirmByTyping` gains
  `autoCapitalize="none"` and an `inputMode`; mobile autocapitalisation is
  exactly the failure `typedMatches` was already made case-insensitive to
  forgive.
- [x] 5.6 The freshness strip as one component, **five flags** — `truncated` is
  orthogonal — and the block/allow rule kept unsplit.
- [x] 5.7 The five `title` attributes move into row disclosures. The Zitadel-read
  one is the important one: it is the only explanation for a failed read.

## 6. Screens

- [~] 6.1–6.18 The Block B screens, per the design reply. Everything that was
  a **defect or a missing safety statement** has landed; what remains is
  layout preference on screens that already reflow with zero overflow at 390,
  plus one item blocked on data nobody records.

  **Landed, and each one turned out to be a claim the product was getting
  wrong rather than a layout to redraw:**
    - **B16 · when the app fails.** There was no `error.tsx` and no
      `not-found.tsx` anywhere in the tree. A render that threw was a blank
      screen with no identifier and no way back. No "Try again": a render that
      threw on the data it was given throws again on the same data.
    - **B11 · the four upstream consoles.** The least undoable writes in the
      product — no rehearsal, no cascade preview, no ledger row — were the
      only ones with no ceremony at all. All four now gate on a tick naming
      what is missing, the consoles carry a standing line, and a failed read
      says whose failure it is.
    - **B15 · Today.** `merge_findings` was inside the headline's arithmetic
      with no block on the page *and* was being dropped by the query mapper,
      so the count had never once been non-zero. Both fixed; the headline went
      17 → 19 against a backend reporting two.
    - **B2 · token shape.** The preview went on claiming "exactly what this
      app would receive right now" while the editor beside it held unsaved
      edits. It now says which shape it is showing.
    - **B14 · member landing**, **B18 · token simulator**, **B8 · requests**,
      **B6 · unexplained access** — see design §7a for the three where the
      design's copy names a fact nobody holds, and the bulk-revoke absence
      that is now stated where an operator looks for it.

  **Deliberate deviations, reasoned in design §7a and §7b:** the count is
  stated once in the selection bar rather than in five sibling verbs; Today's
  blocks do not keep hollow seats at zero; and the rule-delete copy is *not*
  changed to "the access it already caused stays", which would be a lie here —
  Syndra cascades revokes, and the dialog already says so accurately.

  **Blocked, not skipped:** B7's reopened-row sentence — *"Acknowledged on 12
  Aug, then the expiry moved to 30 Sep"* — needs the acknowledgement to record
  *which* expiry was acknowledged. `ExpiryAcknowledgement` carries `by`, `at`
  and `note` and nothing else, so the sentence cannot be computed. Same
  category as §7: a backend field, not a UI change.

  **Layout preference, deliberately not taken:** B3's four-step rule sheet,
  B5's full-height picker and version spine, B10's eight panels as a scrolling
  tab set, B17's map overlay and depth segment. Each of those screens was
  measured at a real 390 with zero horizontal overflow and no undersized
  control; redrawing them is a redesign, not a mobile fix. B12's *"4 new since
  you opened this"* is moot — the audit feed has no live stream, so rows
  cannot insert themselves under a reading thumb.

- [x] 6.19 The dormant sweep states that the size is unknown rather than
  implying zero — `filesSentence` was already right; this pass only confirmed
  it and gave the row a target a thumb can hit.

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

- [x] 8.1 Browser pass at 390 / 744 / 1280 for every route touched, and a
  second pass at a **real** 390 after the selection work — Chrome's window
  floor is ~500px, so `resize_page` alone had been measuring a wider screen
  than it reported. It found five targets no class contract could: a selected
  row whose name still navigated away, a 19px "Select similar", a 42px row
  expander, three 34px duration pills and a 26px sheet grabber.

  **Final sweep, after all of the above:** every route in `nav.ts` plus the two
  boundaries and the member's own screens, at a real 390 — **zero horizontal
  overflow on every one, and no undersized control that is not an inline link
  inside a sentence** (WCAG 2.5.8 exempts those, and all four found are the
  last clause of a paragraph). Light theme verified on the screens this change
  touched, and the over-ceiling bar computed at 4.9:1 light / 9.7:1 dark
  against its own background, which is AA either way.
- [ ] 8.2 The per-route a11y checklist `BIA-36` and `ISC-43` already owe, now
  including touch targets and dynamic type.

## 9. Audit follow-up

Three regressions found by auditing the finished branch. See design §9.

- [x] 9.1 Five mutations held an outcome in state and rendered nothing — the
  welcome-bundle toggle, the bundle working-copy role drop, the bulk
  confirmation-mode verbs, the upstream role editor and the upstream grant
  removal. Each had a `toast.error` before this change, so the replacement had
  removed the only failure signal. All five now render `ActionOutcome` where
  the action ran, covered by `policies/__tests__/bulkOutcome.test.tsx`.
- [x] 9.1a The upstream role dialog also set an `applied` outcome one line
  before `onClose()`, writing a report into a component that was unmounting. It
  now stays open and becomes its result like every other dialog in the product,
  disables its primary so the untracked write cannot be repeated, and relabels
  Cancel to Done — `zitadel/projects/__tests__/roleDialog.test.tsx`.
- [x] 9.2 `autoFocus={!touch}` never suppressed the keyboard: `useMediaQuery`
  answered from an effect, and React applies `autoFocus` at commit. Rewritten
  on `useSyncExternalStore` so the answer is available during the first render;
  `lib/__tests__/useViewport.test.tsx` asserts the field is not focused on a
  touch viewport and still is on a pointer one.
- [x] 9.3 The nav sheet pushed a history entry per render, not per open, and
  never spent the one it pushed. `onClose` moved into a ref and dismissal now
  goes through `history.back()`; five cases in `shell/__tests__/TouchNav.test.tsx`
  cover the poll, both manual dismissals, the back gesture and the
  pick-a-destination exception.

- [ ] 9.4 Promote `@typescript-eslint/no-unused-vars` from warning to error, so
  the next unrendered outcome fails the build rather than scrolling past. Needs
  four unrelated dead bindings cleared first: `onDeleted` in `bundles/page.tsx`
  and `policies/page.tsx`, `stopped` in `AddRolesToBundle.tsx`, `_ack` in
  `BulkDialog.test.tsx`.

## 10. The audit's remainder

Deferred in §9's pass, then asked for. See design §10.

- [x] 10.1 The nav sheet's grabber: 44px target, `tabIndex={-1}` so it stops
  taking the focus the sheet gives on open, and "Dismiss" so both sheets'
  handles answer to one word. Negative margins keep the bar where it was.
- [x] 10.2 `--touch-nav-height` is `calc(68px + env(safe-area-inset-bottom))`.
  It was 76px flat, which put the freshness dock 2px inside the tab bar on a
  notched phone.
- [x] 10.3 `AccessSourceList`'s `+N more` opens on tap instead of carrying the
  other source names in a hover-only `title`.
- [x] 10.4 Offline and degraded share one sticky slot; degraded gets phone
  gutters.
- [x] 10.5 `app/apple-icon.png`, 180×180 — iOS reads no other kind, and the
  installed app had no icon at all.
- [x] 10.6 `type-nav-group` 10.5 → 12.5px and `type-label` 11.5 → 12.5px, with
  a test that reads every named type role out of `globals.css`.
- [x] 10.8 The destructive kebab in `PersonAccess` clears the 44px floor, and a
  brace-aware sweep now catches any raw `<button>` that states a height under
  it. A regex stopping at the first `>` could not: `=>` in a handler ends the
  tag early, which is why the first version of the sweep reported clean.
- [x] 10.9 The raw NUL in `useRowSelection.ts` is `\u0000`, so the line
  is visible to grep again.

- [ ] 10.7 The 22 inline type sizes still under the floor — `text-[11px]`,
  `text-[11.5px]`, `text-[12px]` across 17 components, `ActionOutcome` and
  `WithheldPill` among them. A layout change on seventeen screens; belongs with
  8.2's browser pass, not ahead of it.

## 11. Floor everywhere, ceiling where it bites

- [x] 11.1 Fourteen raw `text-[Npx]` breaches raised to 12.5px, and the floor
  guard extended past `globals.css` to className literals. Decoration is
  exempt and provably so: every sub-floor size left in the tree is on an
  `aria-hidden` element, except `Avatar`'s size map, which states
  `type-floor-exempt`.
- [x] 11.2 `ceiling` wired on Requests and Unexplained access, the two
  select-all surfaces whose endpoints refuse past `services.BulkMaxUsers`.
  Tested at 501 rows on both. Automatic rules and expiring-access have no
  server cap; Converge has a cap but no row selection.

- [ ] 11.3 `visibleCount` / `onSelectVisibleOnly` / `wholeScope` are still
  People-only, so the other four selection surfaces cannot offer "select only
  the N shown" or distinguish a whole-scope selection from a counted one.
- [x] 11.4 `global-error.tsx` — done in 12.2.

## 12. The third ceiling, and the boundary below the boundary

- [x] 12.1 Expiring access gates on people rather than grants. §11 checked the
  per-row acknowledge and missed the bar's own verb, which extends through the
  capped grants endpoint. `SelectionBar` gained `ceilingCount` / `ceilingNoun`
  for the one surface where the counted unit and the capped unit differ;
  narrowing takes whole people so nobody is left half-extended.
- [x] 12.2 `app/global-error.tsx`, for a throw in the root layout itself. It
  imports nothing from the app — enforced by test — and states both themes
  itself, because the shell that would have chosen one is gone.

Still open: 11.3 (selection parity across the other four surfaces) and 8.2 (the
per-route browser pass, now carrying every type change from §10 and §11).
