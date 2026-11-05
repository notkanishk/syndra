# second-pass tasks

Each proposal has its own labelled section. Tags follow `docs/agents/triage-labels.md`.

## P1 · Names link, everywhere

- [x] `NameLink` + `nameHref`; `link` prop (default true) on `UserName`, `ProjectName`, `ResolvedProjectName`, `BundleName`, `RoleRef`; `href` on `UserName`
- [x] Remove the two `PersonLink`s and the A-17 `<Link><Name/></Link>` wrappers
- [x] `link={false}` where a container is the link (row links, buttons, `ArrowLink`, Pending's filter chip)
- [x] Names plain inside `Modal` by default
- [x] Guard: source scan (`names-link.guard.test.ts`) + render-time `NESTED_NAME_LINK`
- [x] Tests: default link, opt-out, dialog default, unresolved name; Pending, Expiring and Roles screens carry no `a a`
- [x] `bun run test && bun run lint && bun run build` green
- [ ] `needs-triage` Teach the name resolver which ids are service accounts so a bare `UserName` opens `/system/service-accounts/:id` (today only rows that call `useAccountHref` do)

Each agent adds its own section.

## P2 P3 P6

- [x] P2: `WELCOME` is the second sentence; `ABOUT` for the calm rung; `headerLine` takes `firstVisit`; `MemberAccess` and `Home` stop inserting a row
- [x] P2 tests: Home calm versus working day with the date line in the same seat; My access title, ladder visible, no extra row; `headerLine` first-visit rungs
- [x] P3: `ShellNotices` seat; `AppShell` stops mounting the banners; `PageHeader`, Home and `MemberAccess` mount the outlet
- [x] P3 tests: order is header, lede, Notice, content; one seat for two headers; headerless fallback; design-system guard updated
- [x] P6: `TopBar` mark and Go to below tablet; `GoToHost` listens for `OPEN_GO_TO`; `SyndraMark` blooms only where visible
- [x] P6 tests: mark and Go to for operators in Basic and Advanced; member has the mark and no Go to; the event; tap floor by token, no tab
- [ ] Look at it on a real phone (board `a20-first-visit-and-phone`): the mark's bloom after sign-in, the bar at 360px with the view switch and avatar beside Go to

## P4 P7

- [x] P4: `ListStates` `head` prop, drawn with rows only
- [x] P4: 15 lists move their `CardColumns` into `head`
- [x] P4: guard test (no `CardColumns` straight above `ListStates`) and rows/empty test
- [x] P7: `ErrorState` four parts, `nextStep`, optional `fine`
- [x] P7: `describeFailure` maps raw network errors to "Syndra's server didn't answer."
- [x] P7: project page and app page keep the seeded name on error
- [x] P7: tests for the four parts, no raw network text, seeded name
- [ ] P7: pass `fine` ("Syndra itself is fine.") from Zitadel-facing pages `ready-for-agent`

Each part adds its own section. Tags per `docs/agents/triage-labels.md`.

## P5 P8

### P5 · one word per state, System group

- [x] `SYSTEM_WORDS` and `zitadelReading` in `components/targets/TargetReading.tsx`; Connected systems, a system's page, `MyStorage` and `PersonStorage` read from it
- [x] Zitadel health line: Answering / Not answering (pips) / Offline (Notice, same word) / Not connected; Check again; `useMissedChecks` counts polls that did not come back
- [x] Zitadel header: no count until it has been read
- [x] Member reads "Answering", not "Online"
- [x] "Waiting on you": one card that holds its place (skeleton, then "Nothing is waiting on you.")
- [x] "Not created by Syndra" in `UnconfirmedChanges`; "by hand"; "Cancel the change"; five EmptyState full stops; connected-systems empty state
- [x] Tests: `SystemWords.test.tsx`, Zitadel page tests (Not answering, Offline after three, Check again, no count while loading), `TargetOverview.test.tsx` (empty and unread), `MyStorage.test.tsx` (Answering)
- [ ] `plain-language.test.ts` guard for "Online" / "Unreachable" as state words and "accounts" as a count word (`needs-triage`: "Online" has legitimate uses in `useOnline`)
- [ ] "people" for every count in the System group (`ready-for-agent`)
- [ ] Offline for Zitadel counts polls the page has seen; a backend `recent` array on `/zitadel/health` would make it survive a reload (`needs-triage`)
- [ ] P11, Technical detail and raw ids (wave 3)

### P8 · the forecast reaches everything

- [x] `FORECAST` gains give, cancel, adopt, mark; copy guide section 5 table and `forecastGuide.test.ts` follow
- [x] `forecastFor(act, roleWaits)`; `useRoleWaits` (own module so surfaces test without the list)
- [x] Forecast seat on Drift (single, bulk), Remove (direct, bundle), Give, bulk apply (`RehearsalDialog`, `bulkForecast`) and Cancel (`CancelQueuedStep`)
- [x] Request rows, Expiring access and the person's Extend say the wait for a role that waits
- [x] `settleIndicators` in `onSuccess` of decide, Drift (single and bulk), lift, give, cancel, withdraw; confirmed by the existing refetch
- [x] Tests: `forecastReach.test.tsx`, `requestForecast.test.tsx`, `RemovalForecast.test.tsx`, `GrantForecast.test.tsx`, `CancelForecast.test.tsx`, `RehearsalForecast.test.tsx`, Drift cases in `UnexplainedAccess.test.tsx`
- [ ] Remove's rail count: only a withdrawn queued grant moves it; a direct revoke's count is left to the refetch (`needs-triage`)
- [ ] Bulk decision and bulk extend rail counts (`ready-for-agent`)

### P9 · the design system tells the truth

- [x] `moves.json` corrected from the code (lib/motion, globals.css utilities, component classes): Checkbox, Radio, Seat, ChoiceRow, AnchoredPanel, ConfirmByTyping, CopyableValue, ViewSwitch, SyndraMark, ChosenChip, SortHead, FilterChip, WaitsMark, ThemeToggle, StatCard, FormPart, SectionPill, ReadLine, SignInRow, CardRow, Card, Notice (breathe), Input, Term, TopBar and the rest of the cards that press or tint; regenerated with `node .design-sync/moves.mjs`
- [x] Numbers checked against code: ArriveList rises 8px (not 10); Modal 98% / 8px / 40ms; lift 6px; popover 4px; drop 10px; copied 900ms; skeleton 200ms; typing warmth 70% at the full name. Finding correction: `Chip` and `Badge` do not move (only `ChosenChip` has `chip-in`); their lines stay still
- [x] ArriveList, GlideIndicator, LeavingRow marked "Internal: never mounted alone" on their floor card
- [x] README (`conventions.md`) gains tints, hover, focus ring, scrim, shadows, tap, layers, radius and motion tokens, and a Moves and Phone paragraph
- [x] `SyndraRoot` provides Next's `AppRouterContext` (a router that goes nowhere): `IntentPrefetchHost` threw in AppShell and every card that mounts it
- [x] AppShell, RoleWordsTable, SignInRow render; TouchNav renders at 390px (`cardMode: single`; the render check still calls it thin, benign: the check's own page is 900px wide and the bar is `tablet:hidden`); new previews RoleWordsTable, SignInRow, ReadGate, FormPart, SectionPill
- [x] `--radius-shell` deleted (unreferenced). `--font-sans` kept and used: `body` now says `var(--font-sans)`, because 48 published boards read `var(--font-sans)` and deleting it would drop their font
- [x] Button names G1 and G16, Notice G7 and the breathing dot, Modal G4 and its phone sheet, FilterBar G18 (card rewritten, preview leads with pills); StatCard preview drops "Paused" and "Unreachable"; `CommandBlock.tsx:159` loses its alpha (the other `warn-ink/[.NN]` on that block are fills and a border, left)
- [ ] Publish the bundle to the design-system artifact and canvas (BRIDGE steps 2-3) `ready-for-human`
- [ ] Task 10 candidate: `CommandBlock` still has `text-warn-ink/[.78]` on its lede and step text (G13) `needs-triage`
