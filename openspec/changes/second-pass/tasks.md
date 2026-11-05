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
