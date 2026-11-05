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

## P10 P14 P15 P19

### P10 · one jump from Basic to Advanced

- [x] `AdvancedLink` (`components/ui/AdvancedLink.tsx`): a plain link that sets the view to Advanced on the click when the view is Basic and the destination is Advanced-only; `isAdvancedOnly(href)` in `lib/nav.ts` reads it from the two rails (Basic's rows win; the System group's dynamic pages are named), query and hash never decide
- [x] `ArrowLink`, `NextStep` and `CardHeaderLink` are `AdvancedLink`; `StatCard`, the Makerspace row, Home's "Checked, nothing waiting" names, its service-account rows and a system's name on the storage card are `AdvancedLink`s
- [x] Home's "Pending changes and Drift are in Advanced." is two links, Pending changes and Drift, each switching the view
- [x] Design system: `AdvancedLink` in `.design-sync` (entry, config, preview, moves row)
- [x] Tests: `AdvancedLink.test.tsx`, `nav-advanced.test.ts`; the Home tests' `ui-view` mocks gain `useUiView`
- [ ] A plain `<Link>` from a Basic surface to an Advanced page written by hand elsewhere is not caught by a guard; `isAdvancedOnly` is the one place to add one (`needs-triage`)

### P14 · one breakpoint, and the phone's number seats

- [x] `sm:` is `tablet:` in `FilterBar`, `Home` and `PersonAccess` (`Modal`'s `sm` is a size, not a breakpoint); a guard in `design-system.test.ts` fails on any `sm:` variant
- [x] The tap floor is `var(--tap)`: the five literals (`users` x2, `TouchNav`, `MemberCatalog` x2) and the same `w-11` seat on Requests, Drift and Expiring; the guard fails on `w-11` and `min-h-[44|48|52px]` (`CopyableValue`'s read-only row excepted). The tab bar's tabs are 44px, not 52
- [x] Roles that wait (two seats), Bundles (Roles, People) and Recent changes take the phone form: the number with its word under the name, the 110px seat from `tablet:`
- [ ] A look on a real phone at 360 and 700px (`ready-for-human`)

### P15 · who you are, on the top bar

- [x] The account chip at tablet and up shows the first name (`firstName` moved to `lib/greeting.ts`, shared with Home); the sheet keeps the full name and address; the phone shows the mark (P6) and the avatar
- [x] "Sound when drift is found" is for operators only (`AccountSheet`)
- [x] Tests: `AccountSheet.test.tsx` (member, first name, no name)

### P19 · Back keeps the rest of its place

- [x] A person's project folds (`PersonAccess`) and the claims editor's open line (`ShapeCard`) are `usePlace`; a stale open line (a row that no longer exists) is ignored (EOY-P5.4)
- [x] `usePlaceScroll` is keyed by path and query: a forward query-only change keeps the scroll and saves it under its own query, and Back to an earlier query returns that query's place (EOY-P5.5); `PlaceHost` reads the query under a `Suspense`
- [x] Requests: a remembered status other than Open is also said as an in-force chip, "Showing declined requests", with the way back to Open
- [x] Tests: `place.test.tsx` (query-only change), `PersonAccessGroups.test.tsx` (folds on Back and not on a fresh visit), `requestPill.test.tsx`
- [ ] Select mode and Pending's other query narrowings are not remembered (`needs-triage`)
