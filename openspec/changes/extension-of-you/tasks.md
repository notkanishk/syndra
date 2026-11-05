# Tasks: an extension of you (A-19)

## A. Wave 1 navigation (P1, P3, P4)

### A1. P1, the name arrives with you
- [x] A1.1 `lib/queries/seed.ts`: `useSeededUser` (People list cache, then name catalog) and `useSeededApp` (Apps list cache); no request
- [x] A1.2 `PersonAccess`: header and breadcrumb on the first frame, outside the skeleton delay; skeleton only under it
- [x] A1.3 `AppTokenScreen`: name, kind, project and breadcrumb on the first frame
- [x] A1.4 Person tab in the URL (`?tab=`), written as a replace; unknown value falls back to Access
- [x] A1.5 Tests: first paint has the name and no skeleton; skeleton only covers the rows; no seed means today's behaviour; tab survives from the URL and is written back; app page the same
- [ ] A1.6 Project and bundle pages seeded from their lists `ready-for-agent`

### A2. P3, Waiting for you to send lands on you
- [x] A2.1 `NextStep person`, passed by `ActionOutcome` and `NotSentYet`; wired on the person page, Give a role and Requests
- [x] A2.2 Pending changes `?person=` as a clearable `FilterChip`; read line keeps "Sending sends all"
- [x] A2.3 Names on Pending changes link to the person (G14)
- [x] A2.4 Tests: the link carries the person; the page narrows; the chip clears and the address is clean; empty person; links
- [ ] A2.5 Pass `person` from the Removal dialog and Roles that wait `ready-for-agent`

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

Owned by the parallel build; it adds its own tasks here.
