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
# Second pass: tasks

Each agent adds its own section.

## P2 P3 P6

- [x] P2: `WELCOME` is the second sentence; `ABOUT` for the calm rung; `headerLine` takes `firstVisit`; `MemberAccess` and `Home` stop inserting a row
- [x] P2 tests: Home calm versus working day with the date line in the same seat; My access title, ladder visible, no extra row; `headerLine` first-visit rungs
- [x] P3: `ShellNotices` seat; `AppShell` stops mounting the banners; `PageHeader`, Home and `MemberAccess` mount the outlet
- [x] P3 tests: order is header, lede, Notice, content; one seat for two headers; headerless fallback; design-system guard updated
- [x] P6: `TopBar` mark and Go to below tablet; `GoToHost` listens for `OPEN_GO_TO`; `SyndraMark` blooms only where visible
- [x] P6 tests: mark and Go to for operators in Basic and Advanced; member has the mark and no Go to; the event; tap floor by token, no tab
- [ ] Look at it on a real phone (board `a20-first-visit-and-phone`): the mark's bloom after sign-in, the bar at 360px with the view switch and avatar beside Go to
# Second pass tasks

## P4 P7

- [x] P4: `ListStates` `head` prop, drawn with rows only
- [x] P4: 15 lists move their `CardColumns` into `head`
- [x] P4: guard test (no `CardColumns` straight above `ListStates`) and rows/empty test
- [x] P7: `ErrorState` four parts, `nextStep`, optional `fine`
- [x] P7: `describeFailure` maps raw network errors to "Syndra's server didn't answer."
- [x] P7: project page and app page keep the seeded name on error
- [x] P7: tests for the four parts, no raw network text, seeded name
- [ ] P7: pass `fine` ("Syndra itself is fine.") from Zitadel-facing pages `ready-for-agent`
