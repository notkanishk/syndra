# A-19 · An extension of you (draft, 5 Nov 2026)

Answer to `Q-19-extension-of-you.md`. Boards: `a19-go-to-and-arriving.dc.html` and `a19-act-in-place-and-keys.dc.html` (Proposed, 5 Nov 2026). New parts: `design/proposed/GoTo.md`, `design/proposed/Kbd.md`. The owner's choices are in §6.

**Read for this:** RULES.md (with G1 to G19), FINAL.md §4, A-05, A-08, A-17 §2.1, A-18, the copy guide §2 (the keeper's voice), `globals.css`, `lib/motion.ts`, `lib/nav.ts`, the shell, the queues, `PersonAccess.tsx`, `MemberAccess.tsx`, `AppTokenScreen.tsx`, and both artifacts (the Design System artifact `WYGTuq4sKKHvUn4QCK3Z8c`, its `project/README.md`, `tokens.css` and component cards; the Syndra Project canvas `Mz163mxndNYBNcKq6ucQpR`, its `canvas.json` and the A-09 to A-18 boards). Nothing was published, edited or commented.

**In one paragraph.** Syndra's rules already describe a product that feels like an extension of its reader: one pattern per job, nothing moves under you, every act leaves a trace where you did it, the keeper speaks only when something changed for you. What is missing is not a new aesthetic. It is the connective tissue between the screens: the name you clicked should already be on the next page when it paints; the first 100ms after a press should say what will happen in the product's own future tense; Back should return you to the exact place; and there should be one way to get anywhere by typing its name. Fourteen proposals follow, each grounded in a file. The first wave is seven of them, all S or M, and none adds a loop, a route transition, or a word the copy guide would cut.

---

## 1 · Findings

Each finding names the file and says what it costs the reader. **clash** means the code contradicts a rule already settled; **gap** means no rule covers it yet.

### 1.1 Flow and continuity
- **The name is lost on the way in.** A People row is a whole-row link (`app/users/page.tsx:625`) that carries nothing. The person page shows a nameless `RowSkeleton` (`PersonAccess.tsx:107-121`) and sets the breadcrumb only after the read (`:145`). The rule says "a page opened from a link already knows the thing's name: show the name while loading" (RULES, second bullet). The app page does the same (`AppTokenScreen.tsx:54-60`). **clash**
- **A person's tab is not in the URL** (`PersonAccess.tsx:130`): reload lands on Access, and Activity cannot be linked or returned to. **gap**
- **"Waiting for you to send →" lands on the whole queue.** `NextStep` defaults to `/governance/pending` (`NextStep.tsx:16`); Extend, Remove, Drift and Roles that wait all use it. Only `PersonStorage.tsx:199` passes `?q=<name>`. On Pending changes the names are plain `UserName` text (`pending/page.tsx:285,502`), so there is no way back to the person either (G14). **clash**
- **Back forgets.** The 10/25/all fold is `useState` (`PaginationRow.tsx:14`), as are Requests' status pill (`RequestsScreen.tsx:136`) and Expiring's open sections (`expiring-access/page.tsx:111`). Scroll relies on the browser. `ArriveList` already knows a return from a first visit (`ArriveList.tsx:21-51`), so the signal exists; nothing uses it to restore. **gap**
- **Dead ends.** "Everything has reached Zitadel." has no next link (`pending/page.tsx:350`). Bulk mode stays on after the bulk dialog closes (`users/page.tsx:520`). Expiring rows do not link the person, unlike Holds and Requests. "Ask to keep it" leaves My access for `/requests` and does not return (`MemberAccess.tsx:438`). Home's "Sent, not seen back · and N more" goes to `/users` unfiltered (`Home.tsx:913`). **gap**
- **A claims draft dies on navigation.** `ShapeDraft` is `useState` (`AppTokenScreen.tsx:111`), with no warning and no memory. **gap**
- **One wording clash on Drift.** A queued revoke shows "Waiting for you to send" beside a detail saying revocations send on their own (`UnexplainedAccess.tsx:1195-1210`). One of them is false; which one is code's to say (Locked). **clash**

### 1.2 Responsiveness
- **The data layer is right for latency hiding and unused for it.** React Query keeps a 30s `staleTime` and a browser-wide cache (`lib/query-client.ts`), so a revisit paints from cache. But no mutation touches the cache: there is no `onMutate`, `setQueryData` or `useOptimistic` anywhere, only `invalidateQueries` on success. The rail's count changes only when the refetch lands. **gap**
- **The first 100ms differ by screen.** Requests' Approve swaps the buttons out at once (`request-decisions.ts:123`) but shows nothing in their place until the response. Expiring's Extend shows the breathing dot (`Button.tsx:84-107`) and the row stays as it was. Both are honest; neither says what is about to happen, although the product has a tense for exactly that: "Will wait for you to send" (RULES, the plan row). **gap**
- **Skeleton strategy is sound** (200ms delay, `states/index.tsx:61`; fade in by `useTintAfterSkeleton`). The gap is only what the skeleton covers: today the whole page, including the name the reader just clicked.

### 1.3 Motion
- **The language is settled and the code is scattered.** `lib/motion.ts` holds four constants, `stillMotion`, `useExitPhase` and `glideTo`. Arrive, Unfold, LeavingRows, Crossfade, GlideIndicator, `useFlashOnChange`, `lift` and `bloom` each live in their own component. Reduced motion is handled in one CSS block (`globals.css:635-690`) and in `stillMotion()`, which is good; the test that binds `lib/motion.ts` to the CSS tokens covers four of the eleven. **gap**
- **No shared element exists between list and detail, and none should.** FINAL §4 lists "a route transition" under Never, and the View Transitions API is unused. Continuity in Syndra is stillness: the press holds on the row until the route commits, and the next page's header paints with the name already in it. §2 P2 says how; the board draws it.
- **The design-system artifact's `tokens.css` carries no motion tokens** (`--ease-*`, `--duration-*`, `--travel-cap`, `--arrive-step` are only in `_ds_bundle.css`), so a board author reading tokens alone sees no motion language. Two of 109 component cards mention reduced motion; eleven mention the phone. **gap**

### 1.4 Getting around
- **No jump surface.** Search is per page, on eight pages. There is no command surface, no global people search, and no global keyboard shortcut; `cmdk` is absent. The phone has a "Go to a page" sheet in Advanced (`TouchNav.tsx:331`), which is the seed of one. **gap**
- **Keys exist on one page only.** Insights' map has `/`, `?`, `t`, `l` and Esc (`MapTab.tsx:213-258`). Elsewhere: Esc on dialogs and bulk, arrows in Select and Segmented. No roving focus in lists. **gap**
- **Memory is partial.** People's filters are in the URL (`lib/people-filters.ts:107`), and so are Roles, Audit, Expiring's window, Pending's `?q`, Insights. Requests' status pill, every sort, every fold and every open section are not. Across visits only the view (cookie), the theme and the drift sound are kept. **gap**
- **Appearance can follow the device, but you cannot ask it to.** `theme.tsx:32-43` mirrors `prefers-color-scheme` only while nothing is stored; the Appearance control offers Dark and Light, so one tap ends the mirroring for good. **gap**

### 1.5 Organic rhythm, layering, both themes
- **Tokens are clean.** Zero hex in components; two `rgba(0,0,0,…)` in `Modal.tsx:426,503` and one `bg-black/55` at `:319`, all the scrim. Light's scrim should be the violet-black its `--shadow-dialog` already uses, not pure black. **gap**
- **No layer scale.** Raw `z-10`, `z-20`, `z-50`, `z-[5]` with a comment about a past `z-60`. **gap**
- **The tap floor (44px) is a comment, not a token.** **gap**
- **Density is one rhythm throughout**, which is right for this product; nothing to add. Light and dark are both complete; dark has no card shadow by design.

### 1.6 The design system as published
- The artifact's `project/README.md` example uses `CardHeaderLink` "See all →", which G2 retired. It lists `accent-note` as a ready-made class, which G19 retired for explanation. `Button` still exports `accentSoft` (G1 retired it). **clash**
- G15 names `ChosenChip` as the one chosen-with-× part; it is not built. `FoldChevron`, `Term` and `CancelQueuedStep` are built but not in the bundle. **clash**
- 22 cards have zero examples (AnchoredPanel, ChoiceRow, SectionPill, Unfold, SortHead, TokenBlock, among them); none says when not to use the part; the `.d.ts` stubs are about 190 bytes each, so a board author reads the source anyway. **gap**
- The canvas's `designSystems` is empty: the bundle still loads from `project/_ds/…`, as BRIDGE says is temporary.

---

## 2 · Proposals, ranked by impact

Each has what, where, why, and a cost (S under a day, M a few days, L a week or more). "Rule" says which standing rule it serves or extends.

### P1 · The name arrives with you · **S/M**
- **What.** A link to a thing seeds the next page with what the row already knows. A People row carries the person's id, name, email and kind; the page's header, breadcrumb and avatar paint from that on the first frame, and the skeleton covers only what the row could not know (the access rows). The same for an app (name, kind, project), a project, a bundle.
- **Where.** `app/users/page.tsx:625` and every `UserName`/`ProjectName`/`BundleName` link: on click, `queryClient.setQueryData(["user", id], partial)` with a `partial: true` mark, so `PersonAccess` renders the header from it and `ReadGate` keeps the skeleton for the rest. `useCrumb` reads the same seed.
- **Why.** It is the rule already written ("show the name while loading"), and it is the whole of shared-element continuity that Syndra permits: the thing you pressed is still there when the page changes. Nothing moves, so nothing can grate.
- **Also.** The person's tab goes into the URL as `?tab=activity` (replace, `scroll:false`), so a reload and a link keep the tab. Structure does not change: the tabs are always the same three.
- **Rule.** Second bullet of RULES; G10; FINAL §4 S7 (skeleton only after 200ms, and now only below the header).

### P2 · The first 100ms, in the future tense · **M**
- **What.** One contract for every act taken in a row. In the first frame: the pressed control shows its busy state (the breathing dot; label and width fixed, as `Button` does). In the same frame, the row's status seat forecasts: amber dot, "Will wait for you to send" (a queued act) or "Will be extended to 26 Nov" (an act that finishes now), in the plan row's tense. On the response, the seat crossfades (S4) to the true word: "Waiting for you to send →" with the link, or "Extended", with the bloom only here. On failure the forecast fades and the buttons return beside the error, as today.
- **Where.** `ActionOutcome` grows a `forecast` phase; `NextStep` grows a `forecast` prop (no link, no arrow, future tense). Callers: `request-decisions.ts:123` (Requests, Home), `expiring-access/page.tsx:643`, `PersonAccess.tsx:757` (Extend), `RemovalDialog`, `HoldsDueForReview`, Drift's `ResolutionDialog`. The rail's count is set on the response (`setQueryData(["governance","indicators"])`), not on the next read; counts never animate (S4), so the number simply changes.
- **Why.** The reader learns in one frame that Syndra heard them and what will happen, without Syndra ever claiming something that is not yet true. Access is observed; the forecast is a sentence about Syndra's own next step, which Syndra does know.
- **Rule.** RULES "A plan row forecasts in the future tense"; A-04 §7 (a step left is said where you took the first one); FINAL S4 (bloom only for something that went well, because of you).

### P3 · "Waiting for you to send →" lands on you · **S**
- **What.** The link carries the person: `/governance/pending?person=<id>`. Pending changes seeds an in-force `FilterChip` ("Priya Nair ✕") from it, with the read line saying "Showing Priya Nair's changes · Send sends all 7". Names on Pending become `UserName` links (G14).
- **Where.** `NextStep.tsx:16` takes `person`; `ActionOutcome` passes it; `pending/page.tsx:76` reads `?person` beside `?q`; `pending/page.tsx:285,502` link.
- **Why.** Today the reader follows the step they were promised and lands on a queue of other people's changes. The link should finish the sentence it started.
- **Rule.** A-04 §7.3; G14; "A cut-off list becomes a violet link to the full list, filtered."

### P4 · Go to: one way to reach anything by its name · **M**
- **What.** A jump surface, `GoTo` (`proposed/GoTo.md`). `/` when no field is focused, or Cmd/Ctrl+K, opens a Modal sm with one Input, "Go to". It matches names Syndra already holds (the name catalog: people, projects, apps, bundles, service accounts) and pages (by title, from `nav.ts`, in the current view only). Results sit in fixed groups in a fixed order: Pages, People, Projects, Apps, Bundles; within a group, A to Z by shown name. Enter opens the first; arrows move; Esc closes. Below tablet the same surface is the existing "Go to a page" sheet (`TouchNav.tsx:331`) with the Input added at its top, in both views.
- **Why.** The operator who knows a name should never need to find the list first. It is the single largest change to how the app feels in the hand, and it uses only parts that exist: Modal, Input, the catalog, `sortByName`.
- **Guards.** Pages shown follow the navigation contract: Advanced leaves appear only in Advanced. No recent-items section (nothing is pinned; alphabetical, always). No search inside a thing (a person's roles): the surface opens pages, and the page has its own search.
- **Rule.** RULES on lists and pickers (alphabetical, nothing pinned); `basic-advanced-ia/design.md`; G18 unchanged (the per-page search stays).

### P5 · Back keeps your place · **S/M**
- **What.** When the reader returns to a list by Back (not by a fresh navigation), it is as they left it: the fold (10/25/all), the open sections, the Requests status pill, and the scroll position. Nothing arrives (already the rule). It is session memory only, keyed by the URL, and it never crosses visits.
- **Where.** `PaginationRow.tsx:14` (`usePaging` reads and writes `sessionStorage["syndra:list:<url>"]`), `RequestsScreen.tsx:136`, `expiring-access/page.tsx:111`, and `AppShell` restoring `#app-scroll` on `popstate` after the list's first paint. `ArriveList.tsx:38` already exposes "this is a return".
- **Why.** "Nothing jumps under the reader" should hold across Back too. Losing the 25-row fold on the way back from a person is the most common small loss in the app.
- **Rule.** Long-list motion 8a to 8d; S11 (never arrive on back).

### P6 · One motion module, one motion page · **M**
- **What.** Gather the movement code under `lib/motion/`: `arrive` (ArriveList and its memory), `unfold`, `leave` (LeavingRows), `crossfade`, `glide` (and the landing flash), `flash` (useFlashOnChange, bloom, lift), `still` (reduced motion). Each exports its still form. The binding test grows to all eleven tokens. The bundle's `tokens.css` gains the motion tokens. Every component card in the bundle gains two generated lines: **Moves** (which verb, and its still form) and **Phone** (what changes below tablet), produced by the resync from a small table rather than written 109 times.
- **Why.** The signature is already defined (A-05, A-08, Motion v2). What makes it feel like one hand is that every surface reaches for the same seven verbs and nobody re-implements a fade. The docs gap (two cards mention reduced motion) is where the next inconsistency will come from.
- **Rule.** FINAL §4 in full; A-08 §5.

### P7 · Keys · **M** (second wave)
- **What.** A small, fixed set, shown once on a Keys sheet (Modal sm) opened by `?`: `/` Go to · `?` Keys · Esc close or leave Select · in a list, ↑ ↓ move between rows and Enter opens (roving focus on the row link; the row's own buttons stay on Tab) · in a dialog, Enter on the act. No letter shortcuts for verbs (an accidental Revoke is not calm). Insights keeps `t` and `l` on its own sheet. A `Kbd` part (`proposed/Kbd.md`) draws a key, used only on the Keys sheet and in the Go to footer, never on a button.
- **Where.** `AppShell` owns the two global keys; `ArriveList` rows take the roving index.
- **Why.** Keyboard-first is how a tool becomes an extension of the hand; four keys are enough, and they are the four every tool uses.

### P8 · Your last view · **S** (owner's taste, Q6)
- **What.** Each list remembers its sort and its window pill (Expiring 14 | 30, Requests' status) in `localStorage["syndra:view:<list>"]`, and applies it when the list is opened from the rail with no filters in the URL. The in-force pill or sort arrow says so; nothing is hidden silently. Search text and FilterChips are never remembered.
- **Why.** A reader who always works the 30-day window should not re-choose it every morning.
- **Trade-off.** Two people at one desk see a list differently. The pill is visible, so this is a preference, not a surprise. Recommended, but it is the owner's.

### P9 · Anticipating the next step · **S**
- **What.** On pointer intent (80ms hover) or focus on a row link or a rail item, prefetch the page's data (`queryClient.prefetchQuery`); Next already prefetches the route's code. Combined with P1, the next page is usually complete on its first frame.
- **Why.** Latency hidden before it exists. No visible change, so nothing to grate.

### P10 · Dead ends closed · **S each**
1. "Everything has reached Zitadel." gains an ArrowLink "Change history →" (G9 allows one action).
2. Bulk mode ends when a bulk dialog closes on its result; Select is a mode, not structure (Q9).
3. Expiring rows link the person, as Holds and Requests do.
4. "Ask to keep it" opens the ask dialog on My access (route-asking's dialog, mounted there); the result "Asked" stays in the row's seat, and the card does not move.
5. Home's "Sent, not seen back · and N more" lands on People filtered (`?attention=unseen`), per A-07 §32.
6. The claims draft is kept in `sessionStorage` per app; on return the editor says "You have an unsaved draft from 14:02." with Discard beside Save. "Preview for" remembers its last person per app.

### P11 · Appearance follows the device · **S**
- **What.** The Appearance Segmented gains a third option, "Device", which is what `theme.tsx` already does when nothing is stored; choosing it clears the stored theme. At a device change, S14 crossfades the tokens (260ms, nothing moves).
- **Where.** `AccountSheet.tsx`'s Appearance row, `lib/theme.tsx:61`. One write place, as today.
- **Why.** The app matching the device's day is the quietest "extension of you" there is.

### P12 · Scrim, layers and the tap floor as tokens · **S**
- `--scrim` in both themes (dark: the black 0.55 used today; light: the same violet-black as `--shadow-dialog`, at 0.35), replacing `bg-black/55` and the two `rgba` in `Modal.tsx`.
- `--layer-bar`, `--layer-pill`, `--layer-popover`, `--layer-dialog` for the four z-index steps that exist.
- `--tap: 44px` for the touch floor, replacing the comment.
- All in `globals.css` only; `tokens.json` picks them up on resync.

### P13 · Design-system hygiene · **S** to **M**
- README example drops "See all →" and `accent-note`; `accentSoft` is deleted (A-17 already lists its two callers).
- Build `ChosenChip` (G15); bundle `FoldChevron`, `Term`, `CancelQueuedStep`.
- The 22 zero-example cards get one example each; every card gets a one-line "Not for" (when not to use it) and the two generated lines from P6.
- `tokens.css` gains motion tokens (P6).

### P14 · Drift's queued revoke says one thing · **S**
- Code decides whether a Drift revoke waits for the operator or sends by itself (it is a correctness fact, Locked), and `ResolutionDialog` plus the row's outcome say that one thing. If it sends by itself, the row reads "On its way · Revocations send on their own", neutral dot, no link; if it waits, the NextStep stands and the detail line goes.

### Deliberately not proposed
- Shared-element morphs between list and detail (a route transition is Never).
- A toast system (every outcome lives where the act was; the comments in `ActionOutcome.tsx:19` are right).
- Recent or pinned items in Go to (nothing is pinned; alphabetical, always).
- Letter shortcuts for verbs, chords, or a shortcut hint on buttons.
- Density settings, compact mode, or a second rhythm.
- Any new loop, any count that ticks, any welcome tour.

---

## 3 · The first wave

Recommended: **P1, P2, P3, P4, P11, P12, P14**, in that order. Together they are about two weeks of UI work, add one component (GoTo), no loop, no route transition, and seven strings. They change what the reader feels in the first second of every navigation and the first 100ms of every act, which is where "an extension of you" lives.

Second wave: P5, P6, P7, P10. Third: P8, P9, P13.

---

## 4 · How the boards read

**`a19-go-to-and-arriving`** (App screens page, right of A-18, 1600 wide)
1. Go to: dark and light at 1440 (the operator's Home behind, the sheet with "pri" typed and the groups), and the phone sheet at 390.
2. The name arrives: a person's page on its first frame, today (nameless skeleton) beside proposed (header, avatar, tabs and breadcrumb set; skeleton only below), dark and light, and the phone.
3. Landing on you: Pending changes opened from "Waiting for you to send →", with the person's chip in force and the names linking.

**`a19-act-in-place-and-keys`**
1. The first 100ms: one access row in four moments (at rest, pressed and busy, forecasting, true word), dark and light, and the same on a phone.
2. Keys: the Keys sheet with the Kbd part, dark and light.
3. Appearance: the Account sheet's Appearance row with Device.
4. Back keeps your place: a list returned to at 25 rows, with its pill at the top.

Both boards mount the real parts from `window.Syndra`. Kbd and the Go to result rows are drawn inline and say so (BRIDGE: a part the system lacks is drawn inline until built).

---

## 5 · For code (once the owner answers)

1. **P1:** `seedFromRow(kind, partial)` in `lib/queries/seed.ts`; `PersonAccess`, `AppTokenScreen`, `ProjectPage`, `BundlePage` render headers from a partial; `useCrumb` reads it; `?tab=` on the person page. Tests: header paints with the seed and no fetch; skeleton covers only the rows; no seed means today's behaviour.
2. **P2:** `ActionOutcome` phase `forecast`; `NextStep forecast`; copy table of forecasts per act (`lib/outcome.ts`); `setQueryData` on indicators from each mutation's `onSuccess`. Tests rung by rung: forecast appears within the same tick as `isPending`; failure returns the buttons; bloom only on the true word.
3. **P3:** `?person=` on Pending; `FilterChip`; `UserName` links. Test: the link from a person's row lands filtered.
4. **P4:** `components/ui/GoTo.tsx` per `proposed/GoTo.md`; `AppShell` key handling; `TouchNav` sheet gains the Input. Tests: `/` ignored inside a field; Advanced pages hidden in Basic; groups fixed, names sorted by `sortByName`.
5. **P11:** `theme.tsx` gains `"device"`; the Segmented gains the option.
6. **P12:** three token families in `globals.css`; `Modal.tsx` reads them.
7. **P14:** one fact, one string.
8. **Copy guide:** add the forecast tense table to §5 (what every action carries) and "Go to" to §6 vocabulary.

---

## 6 · Questions for the owner

Each "yes" accepts the recommendation.

1. **The name arrives with you (P1): seed the next page from the row, and keep the person's tab in the URL?** *Recommended: yes.*
2. **The first 100ms (P2): the seat forecasts in the future tense ("Will wait for you to send", amber dot) while the request is in flight, then crossfades to the true word?** *Recommended: yes. It is the plan row's own tense, and Syndra never says something happened before it did.*
3. **"Waiting for you to send →" filters Pending changes to that person, and the names there link (P3)?** *Recommended: yes.*
4. **Go to (P4): `/` and Cmd/Ctrl+K open one jump surface everywhere; on a phone it is the Go to sheet with an Input at the top, in both views?** *Recommended: yes.*
5. **Back keeps your place (P5), for this session only, never across visits?** *Recommended: yes.*
6. **Your last view (P8): remember each list's sort and window pill across visits, never its search text?** *Recommended: yes, as a preference the pill makes visible. This one is your taste.*
7. **Appearance gains "Device" (P11)?** *Recommended: yes.*
8. **Keys (P7): `/`, `?`, Esc, and arrows between rows with Enter to open; no letter shortcuts for verbs?** *Recommended: yes, second wave.*
9. **Select mode ends when a bulk dialog closes on its result (P10.2)?** *Recommended: yes.*
10. **"Ask to keep it" asks on My access and stays there (P10.4)?** *Recommended: yes.*
11. **"Everything has reached Zitadel." links to Change history (P10.1)?** *Recommended: yes.*
12. **The first wave is P1, P2, P3, P4, P11, P12, P14?** *Recommended: yes.*
