# Tasks: an extension of you (A-19)

## A. Wave 1 navigation (P1, P3, P4)

### A1. P1, the name arrives with you
- [x] A1.1 `lib/queries/seed.ts`: `useSeededUser` (People list cache, then name catalog) and `useSeededApp` (Apps list cache); no request
- [x] A1.2 `PersonAccess`: header and breadcrumb on the first frame, outside the skeleton delay; skeleton only under it
- [x] A1.3 `AppTokenScreen`: name, kind, project and breadcrumb on the first frame
- [x] A1.4 Person tab in the URL (`?tab=`), written as a replace; unknown value falls back to Access
- [x] A1.5 Tests: first paint has the name and no skeleton; skeleton only covers the rows; no seed means today's behaviour; tab survives from the URL and is written back; app page the same
- [x] A1.6 Project and bundle pages seeded from their lists (`useSeededProjectName`, `useSeededBundleName`: list cache, then name catalog; name only)

### A2. P3, Waiting for you to send lands on you
- [x] A2.1 `NextStep person`, passed by `ActionOutcome` and `NotSentYet`; wired on the person page, Give a role and Requests
- [x] A2.2 Pending changes `?person=` as a clearable `FilterChip`; read line keeps "Sending sends all"
- [x] A2.3 Names on Pending changes link to the person (G14)
- [x] A2.4 Tests: the link carries the person; the page narrows; the chip clears and the address is clean; empty person; links
- [x] A2.5 Removal dialog passes `person` (the direct and the bundle outcome). Roles that wait is role-scoped, not person-scoped, so its link stays the whole queue `needs-triage: narrow it by role?`

### A3. P4, Go to
- [x] A3.1 `lib/go-to.ts` (pure matching, fixed groups, A to Z by `sortByName`, view and role rules) and `useNameCatalog` (cache read, no request)
- [x] A3.2 `components/ui/GoTo.tsx` (`GoTo`, `GoToPanel`) and `components/ui/Kbd.tsx`
- [x] A3.3 `GoToHost` in `AppShell`: `/` and Cmd/Ctrl+K, never while typing, never over another dialog
- [x] A3.4 Phone: `NavSheet` gains the Input; page list stays until something is typed
- [x] A3.5 `ui/.design-sync`: entry, config (map, `dtsPropsFor`, override), previews, NOTES
- [x] A3.6 Tests: keys open it and not while typing; alphabetical groups; Basic versus Advanced versus member; Enter navigates; Esc closes; combobox and listbox roles; the phone sheet
- [ ] A3.7 Resync and publish the design bundle with GoTo and Kbd `ready-for-human: needs Chrome and a publish`
- [ ] A3.8 A phone entry for Go to in Basic's tab bar, if the owner wants one `needs-triage`

## B. First moment and system tokens (P2, P11, P12, P14)

- [x] EOY-F1 P2: `ActionForecast`, `FORECAST` words, `forecasts` in `useRequestDecisions`; wired in Requests, Home, a person's Requests, Expiring Extend, person Extend, Holds lift.
- [x] EOY-F2 P2 tests: forecast while pending (breathing amber dot, no bloom, no cache write), crossfade to the true word with the only bloom, honest error with buttons back. `test-utils/seat.ts` `trueSeat()` for tests that want the decision.
- [x] EOY-F3 P11: `choice` and `"device"` in `lib/theme.tsx`; third Segmented option in `AccountSheet`.
- [x] EOY-F4 P11 tests: Device clears storage, follows the OS live, crossfades at a change, stops on Dark/Light.
- [x] EOY-F5 P12: `--scrim`, `--layer-*`, `--tap` in `globals.css` (both themes for the scrim); `Modal`, `TouchNav`, `Anchored`, `SelectionBar`, `SectionNav`, `ConvergeEntitlements`, `AppShell` converted.
- [x] EOY-F6 P12 tests: tokens present in both themes; no component uses a raw z-index or black overlay.
- [x] EOY-F7 P14: traced the backend (`handleRevokeDrift` enqueues `revoke`; `DrainRevocations` claims it); one wording, `REVOKE_SENDS_ITSELF`.
- [x] EOY-F8 P14 tests: sends-itself wording and the waits case.
- [x] EOY-F9 Copy guide section 5 carries the forecast tense table; `lib/__tests__/forecastGuide.test.ts` holds it to `FORECAST`.
- [ ] EOY-F10 Forecast in dialogs and `setQueryData` on the rail indicators, if the owner wants counts to move at the answer. `needs-triage`
- [ ] EOY-F11 `--tap` is declared but components still write `min-h-[44px]`; convert as touched. `ready-for-agent`

## C. Wave 2 · motion and keys (P6, P7, P10)

The owner approved all of it on 5 Nov 2026 (Q8, Q9, Q10, Q11). P5, P8, P9 and P13 are other builds.

### C1. P6, one motion module, one motion page
- [x] C1.1 `ui/src/lib/motion/` holds the motion code: `still` (reduced motion, exit phase, token durations), `glide` (+ `glide-indicator`), `arrive`, `unfold`, `leave`, `crossfade`, `flash`, behind one `index.ts`. Moved, not rewritten: the components' own tests pass unchanged. `bloom` and `lift` are CSS classes and stay in `globals.css`.
- [x] C1.2 The binding test grows to every `--duration-*` (7, with `PRESS_MS`, `BREATHE_MS`, `GLIDE_MS` added) and every `--ease-*` (6): `src/lib/motion/__tests__/motion.test.tsx`.
- [x] C1.3 Moves and Phone lines on every card, generated from `ui/.design-sync/moves.json` by `moves.mjs` into each preview and `MOVES.md`; stale output fails a test.
- [x] C1.4 Motion tokens: none missing from `globals.css`. The gap is the design-system artifact's `tokens.css`, which is a publish step.
- [ ] C1.5 Resync and publish the bundle with the lines, `GoTo`, `Kbd` and `KeysSheet` `ready-for-human: needs Chrome and a publish`
- [ ] C1.6 `tokens.css` of the Design System artifact gains the motion tokens when it is next published `ready-for-human`

### C2. P7, keys (Q8)
- [x] C2.1 `lib/keys.ts`: the key table, `typingIn`, `otherDialogOpen`, roving focus (`useRovingRows`, `rowFocused`, `moveBetweenRows`). `/` stays Go to's.
- [x] C2.2 `KeysSheet` (Modal sm, `Kbd`) and `KeysHost` in `AppShell`: `?` when nothing is typed into and nothing else is open. Insights' map keeps `?` for its own sheet and takes the event first.
- [x] C2.3 `ArriveList` carries the roving index: one `[data-row]` link per list is in the Tab order, ↑ ↓ move, Enter is the link's own press, a row's buttons stay on Tab. Marked on People, Projects, Apps, Roles and a role's holders.
- [x] C2.4 Esc closes things: a Modal's own, Select's (People), and the Keys sheet.
- [x] C2.5 Tests: `?` opens and not while typing, not over a dialog, not with a modifier; no letter verbs; arrows, ends, Tab stop, Enter, fields and buttons left alone.
- [ ] C2.6 Arrow rows on the queues (Pending, Requests, Expiring, Holds). Their rows are not whole-row links, so the person link inside each would carry `data-row`. `needs-triage`

### C3. P10, six dead ends closed
- [x] C3.1 "Everything has reached Zitadel." links to Change history (Q11).
- [x] C3.2 Select ends when a bulk dialog closes on its result, on People and on Expiring; `BulkDialog` `onClose(applied)`. Closing without applying keeps the selection (Q9).
- [x] C3.3 Expiring rows link the person: already true through `WhoSeat`; now held by a test.
- [x] C3.4 "Ask to keep it" is a button that opens the ask on My access; once asked, "Asked to keep it" takes the button's seat (Q10).
- [x] C3.5 Home's "Sent, not seen back" opens People at `?attention=unseen`. Backend: `unseen_count` on the People list from the same read as the card (`UnconfirmedWriteSummary.Users`, not sent on the indicators payload).
- [x] C3.6 The claims draft is kept in `sessionStorage` per app, restored with "You have an unsaved draft from 14:02." beside Discard; "Preview for" remembers its person per app and falls back if they are gone.
- [x] C3.7 Tests for each.
## C. Wave 2 · place (P5, P8, P9)

- [x] EOY-P5.1 `lib/place.ts`: `usePlace` (sessionStorage, keyed by path and query plus the name, restored only when `returnedToThisPage()`; every mount writes, so a fresh visit overwrites and nothing crosses visits), `usePlaceScroll` (the app scroller, saved on scroll, handed back once the page is tall enough, ended by wheel, touch or key), all storage reads and writes in try/catch.
- [x] EOY-P5.2 Wired: `usePaging` (the 10/25/all fold, optional third argument names a second list on a route), Expiring's open sections, scroll via `PlaceHost` in `AppShell`. Section-nav position follows the restored scroll. The Requests status and Expiring window pills return by the P8 memory below.
- [x] EOY-P8.1 `useSort(initial, { list, keys, when })` and `usePill`: sort and pill kept in `localStorage["syndra:view:<list>"]`, only after the reader's own press; a stale key from an older build is ignored; search text and filter chips are never stored. Wired on People (not when a filter or link chose the view), Requests (sort and status), Expiring (sort and window; a link's `?within=` wins), Projects, Apps, Roles, Roles that wait, Operations, Drift, Withdrawn.
- [x] EOY-P9.1 `IntentPrefetchHost` in `AppShell`: 80ms hover or focus on an in-app link warms the Next route and, for `/users/<id>` and `/applications/<id>`, the page's first read (`prefetchUserAccess`, `prefetchAppClaimShape`); once per address; nothing with Data Saver or a 2g connection; a touch is not a hover.
- [x] EOY-P5.3 Tests: `lib/__tests__/place.test.tsx` (Back restores fold, open section and scroll; a fresh visit does not; route-keyed; never in localStorage; sort and pill persist and search does not; stale and unreadable values; storage that throws), `shell/__tests__/IntentPrefetchHost.test.tsx`.
- [ ] EOY-P5.4 Place for the other open folds (`Unfold` owners other than Expiring, the claims editor's tab) `ready-for-agent`
- [ ] EOY-P5.5 Scroll is keyed by path at mount; a query-only change on the same page keeps its scroll rather than keying its own `needs-triage`
- [ ] EOY-P9.2 Prefetch the other detail pages (project, bundle) once their reads have a shared fetcher `ready-for-agent`
- [x] EOY-F11 Every touch floor is `var(--tap)` (`min-h-`, `min-w-`, `h-`, `size-`, `before:h-`); `design-system.test.ts` fails on a typed `44px` or `11`.

## C. Wave 3 · hygiene (P13)

- [x] P13.1 README example (`ui/.design-sync/conventions.md`) drops "See all" and `accent-note`; the `accent-note` utility is deleted from `globals.css` (no caller).
- [x] P13.2 `accentSoft` is not a Button variant in code; its only callers were the Button and ButtonLink previews, now removed.
- [x] P13.3 `ChosenChip` already lives in `ui/Badge.tsx` (G15; both role pickers and the service-account owner use it); now in the bundle map with a preview, and `ChosenChip.test.tsx`.
- [x] P13.4 `FoldChevron` joins the bundle (entry, map, preview). `Term` and `PushedStep` (CancelQueuedStep) were already in.
- [x] P13.5 Nine zero-example cards get previews, each saying its still form where the part moves: Seat, CopiedSeat, Crossfade, Ticks, TokenBlock, CheckTrail, RunLabel, Tinted, UnfoldOnArrival (plus ChosenChip and FoldChevron).
- [ ] P13.6 Nine cards still have no preview (ReadGate, ArriveList, GlideIndicator, LeavingRow, FormPart, SectionPill, RoleWordsTable, SignInRow, GoToPanel): they need seeded data, a route or a floor card `needs-triage`
- [ ] P13.7 The per-card "Not for" line and the two generated lines (Moves, Phone) come from the resync tool and its table, not from the repo; they wait on P6's motion table `ready-for-human: needs the resync`
- [ ] P13.8 `tokens.css` motion tokens (P6, second wave) `blocked: P6`
- [ ] P13.9 Resync and publish the bundle `ready-for-human: needs Chrome and a publish`
