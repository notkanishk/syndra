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

- [x] EOY-F1 P2: `ActionForecast`, `FORECAST` words, `forecasts` in `useRequestDecisions`; wired in Requests, Home, a person's Requests, Expiring Extend, person Extend, Holds lift.
- [x] EOY-F2 P2 tests: forecast while pending (breathing amber dot, no bloom, no cache write), crossfade to the true word with the only bloom, honest error with buttons back. `test-utils/seat.ts` `trueSeat()` for tests that want the decision.
- [x] EOY-F3 P11: `choice` and `"device"` in `lib/theme.tsx`; third Segmented option in `AccountSheet`.
- [x] EOY-F4 P11 tests: Device clears storage, follows the OS live, crossfades at a change, stops on Dark/Light.
- [x] EOY-F5 P12: `--scrim`, `--layer-*`, `--tap` in `globals.css` (both themes for the scrim); `Modal`, `TouchNav`, `Anchored`, `SelectionBar`, `SectionNav`, `ConvergeEntitlements`, `AppShell` converted.
- [x] EOY-F6 P12 tests: tokens present in both themes; no component uses a raw z-index or black overlay.
- [x] EOY-F7 P14: traced the backend (`handleRevokeDrift` enqueues `revoke`; `DrainRevocations` claims it); one wording, `REVOKE_SENDS_ITSELF`.
- [x] EOY-F8 P14 tests: sends-itself wording and the waits case.
- [ ] EOY-F9 Copy guide: add the forecast tense table to section 5 (`plain-language-copy/design.md`). `ready-for-agent`
- [ ] EOY-F10 Forecast in dialogs and `setQueryData` on the rail indicators, if the owner wants counts to move at the answer. `needs-triage`
- [ ] EOY-F11 `--tap` is declared but components still write `min-h-[44px]`; convert as touched. `ready-for-agent`
