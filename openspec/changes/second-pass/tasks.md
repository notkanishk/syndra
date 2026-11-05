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
- [x] P11, Technical detail and raw ids (see P11 below)

### P8 · the forecast reaches everything

- [x] `FORECAST` gains give, cancel, adopt, mark; copy guide section 5 table and `forecastGuide.test.ts` follow
- [x] `forecastFor(act, roleWaits)`; `useRoleWaits` (own module so surfaces test without the list)
- [x] Forecast seat on Drift (single, bulk), Remove (direct, bundle), Give, bulk apply (`RehearsalDialog`, `bulkForecast`) and Cancel (`CancelQueuedStep`)
- [x] Request rows, Expiring access and the person's Extend say the wait for a role that waits
- [x] `settleIndicators` in `onSuccess` of decide, Drift (single and bulk), lift, give, cancel, withdraw; confirmed by the existing refetch
- [x] Tests: `forecastReach.test.tsx`, `requestForecast.test.tsx`, `RemovalForecast.test.tsx`, `GrantForecast.test.tsx`, `CancelForecast.test.tsx`, `RehearsalForecast.test.tsx`, Drift cases in `UnexplainedAccess.test.tsx`
- [ ] Remove's rail count: only a withdrawn queued grant moves it; a direct revoke's count is left to the refetch (`needs-triage`)
- [ ] Bulk decision and bulk extend rail counts (`ready-for-agent`)

### P18 · since you were last here (owner, Q9, 5 Nov 2026)

- [x] Migration `000076_last_sign_in`: `person_first_sign_in` gains `last_sign_in_at` (NOT NULL) and `previous_sign_in_at`, with the `person_sign_in_order` coherence check (current >= first; previous between first and current). One row, two timestamps. Backfill: current starts as the person's latest act in Syndra (audit entry or request)
- [x] `db.RecordSignIn` replaces `RecordFirstSignIn`: each sign-in moves current into previous; a second call within a minute is the same sign-in and moves nothing
- [x] `POST /me/first-sign-in` also answers `previous_sign_in_at` (null on the first). No new route, so the proxy allowlist and router guard are unchanged. The callback puts it in the session as `previousSignIn`
- [x] `lastHere()` in `lib/greeting.ts`: yesterday, a weekday under a week, a date beyond; nothing for today
- [x] Home: calm guidance "Since Tuesday, nothing came in." (no request, any status, since); working headline "Three things need you, two of them new since Tuesday." (a block is new when a request or a joiner in it arrived since). Counts Syndra's records of arrivals, never access
- [x] My access: calm rung "Nothing has changed since you were last here, on Tuesday." only when the changes feed was read and holds nothing newer; every rung above it still speaks first
- [x] Tests: live-DB (previous and current shift, first has none, retry inside the window moves nothing, order guard, down undoes up; settle window mutation-checked); `lastHere`, `headerLine`, session cookie, Home
- [ ] Deviation: Home's "nothing came in" goes in the calm panel's guidance, under the unchanged title "Nothing here needs you.", rather than replacing it
## P12 · Insights to the rules

- [x] Finding chips keep five seats (drift, waiting for you to send, single routes, unused, overlaps); the count is a hollow Badge at zero, the chip is disabled then, and no number is shown before the read
- [x] Unused nodes recede (canvas ground, line-strong edge, solid); "N more" is solid; dashed keeps meaning NotOpenYet
- [x] A warn or danger answer is one surface with a Notice-toned line (StatusDot plus tone text on the eyebrow), not a tinted box
- [x] Dots are StatusDot (`ToneDot`; `StatusDot` gains `fill` for the accent shades a pin needs)
- [x] "Back" is "Back to the map"; "Pin" and "Pin this" are "Keep this answer"
- [x] "Showing 3 Oct" is a FilterChip with "Back to now" as its clear; the map's Keys pill is a Button
- [x] Trends: ReadFreshness; a failed recent-use read says "Couldn't read recent use." apart from "Reading recent use."; all-clear lines take the resolved dot
- [x] Below tablet: the lens Select, the answer above, then the nodes as a plain list per lane with observed people counts (`PhoneNodes`); no number where Zitadel was not read
- [x] Tests: `p12.test.tsx`, phone list, Trends freshness and failed read, chip seats; `bun run test && bun run lint && bun run build` green
- [ ] `needs-triage` Timeline track is still hand-drawn over a native range (no shared slider part exists)
- [ ] `needs-triage` Drift and Expiring all-clear lines (refinement-sweep L4) were not re-checked here; Insights has none left
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
## P11

### P11 · the System group in words, part one (built 5 Nov 2026)

- [x] `TechnicalDetail` (+ `TechnicalPayload`) in `components/ui/TechnicalDetail.tsx`: FoldChevron, Unfold, closed by default, `placeKey` keeps it open on Back; in `.design-sync` (entry, config, `previews/TechnicalDetail.tsx`, moves row)
- [x] Incoming events: an opened row is named facts (who, what, from, when, what Syndra did); the raw type (CopyableValue) and JSON sit under Technical detail
- [x] A system's page: operation ids leave the Capabilities rows for one fold; the log-finding fold is the same part; "Unknown account" instead of an id used as a name (binding conflict, Dormant accounts)
- [x] Mappings: the row says "Gives the fabrication group"; the role key and `field → value` are folded; dialog ledes lose `field = value`; Version history and Merge findings say "group", "home folder" (`fieldWords`), never the field's own name
- [x] Claims delivery: "Syndra expected something other than what Zitadel has." with expected and found folded; an unknown app's client id is a CopyableValue
- [x] Zitadel: the signing key's commands sit under Technical detail, the row says "Rotate the key on the Syndra server. The person who runs it has the command."; grant ids and project ids are folded
- [x] Tests: `TechnicalDetail.test.tsx`, `SystemRawGuard.test.ts` (fails if a System file dumps JSON, prints an id as bare mono, or uses an id as a name), and the Operations, Zitadel, TargetOverview, Mapping, Claims delivery tests now open the fold before looking for a raw value
- [ ] Audit: no raw handle is shown today (A-17 G10); nothing moved there `needs-triage`
- [ ] `RoleRef` still prints the role key beside the project name outside the System group; System-group uses should read the role's display name `ready-for-agent`
- [ ] Signing-key replacement: the owner chose a hands-off flow where Syndra rotates the key itself; a separate backend and UI change, not the four-step manual guide on the board `ready-for-human`
- [ ] `ErrorState` Technical detail for a failure body (status, path) `needs-triage`
