# Tasks — Extension of you (A-19)

Task IDs prefixed `EOY-`.

## Wave 1 · feel

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
