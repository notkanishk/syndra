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

## P13 P16 P17

### P13 · the claims page's leftovers

- [x] Client ID row is Advanced only (`AppTokenScreen`)
- [x] `FoldChevron` on every claim line (`ShapeCard`)
- [x] Cancel in the footer's `cancel` slot, ghost, in the copy, save and roles sheets (`ShapeSheets`, `ShapeCard`)
- [x] A failed re-run keeps the prior preview receded and inert, with a danger Notice that names its age (`ClaimPreview`)
- [x] The page error keeps the seeded header, meta included (`SeedHeader`)
- [x] Tests: `AppClaims.test.tsx` (client id by view, failed re-run keeps the preview)
- [ ] TrueNAS offer and Debug mode's copy: neither names Advanced today; the offer's link to an Advanced page switches the view once `AdvancedLink` (P10) lands in `ArrowLink` (`blocked` on P10)

### P16 · Pending's narrowings become one

- [x] `lib/pending-filter.ts`: one `usePendingFilters` (read, clear, drop the URL parameter) and one `narrow`; `?q=` stays the search
- [x] One FilterChip per filter in force; read line "Showing Priya Nair's 2 changes. Sending sends all 7."; search alone keeps "N of the M changes match."
- [x] The three `NextStep` callers carry a person or a role: `CancelQueuedStep` (when every kept change is one person's), `RolePickerOne` (the role), `PersonStorage` (the person)
- [x] Tests: `person.test.tsx` (the combined line, each chip clears only its own parameter), the three callers' link assertions

### P17 · the small sweep

- [x] Lone dashes become "Not yet" or "None" (Connected systems, `PersonStorage`, Bundles, Roles that wait)
- [x] "and N more" is a violet link: Change history to the one edit (which lists every write), the storage card's hold to the system page
- [x] Keys sheet: "In a list, the arrows move between rows." and a Go to chord row printed with this keyboard's modifier
- [x] Go to placeholder by session (member "A page", Basic without bundles, Advanced with)
- [x] Role page's raw key leaves Basic; bundles lede loses its third sentence
- [x] Tests: `GoTo`, `KeysHost`, cascades, bundles list, role page
- [ ] "Nothing matches that." for every emptied search (`ready-for-agent`: left to code's taste by A-20, not unified here)
