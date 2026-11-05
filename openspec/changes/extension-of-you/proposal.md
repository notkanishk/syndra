# Extension of you (A-19)

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-19-extension-of-you.md`. The owner answered "all yes" to every question in section 6 on 5 Nov 2026.

## Wave 1 · feel (P2, P11, P12, P14)

Built on `feat/a19-wave1-feel`. Another section ("Wave 1 · flow", P1/P3/P4) is added by its own branch.

- **P2, the first 100ms in the future tense.** While a write is in flight the seat forecasts in the plan's tense: an amber breathing dot and "Will be approved", "Will be declined", "Will be extended by 90 days", "Will be lifted". It crossfades to the true word on the server's answer; only the true word blooms. A failure shows the honest error with the buttons back; there is nothing to roll back. Forecasts are words, not counts: no cache is written ahead of the server, so "observed from Zitadel" stays true. The shared seam is `ActionForecast` (`components/ui/ActionOutcome.tsx`), the `FORECAST` table (`lib/outcome.ts`) and `forecasts` in `useRequestDecisions` (Requests, Home, a person's Requests tab). Expiring Extend, a person's Extend and Holds' Lift use React Query's `isPending`.
- **P11, Appearance "Device".** A third option that clears the stored theme and follows `prefers-color-scheme` live, crossfading S14 at a device change. `useTheme()` gains `choice` (`"device" | "light" | "dark"`); `setTheme` accepts `"device"`. One write place: `lib/theme.tsx`.
- **P12, tokens.** `--scrim` (dark `rgba(0,0,0,.55)`, light `rgba(60,40,120,.35)`), `--layer-bar/pill/popover/dialog` (20/30/60/50), `--tap: 44px`, all in `globals.css`. The sheet drag thins the scrim with `--scrim-keep`. The navigation sheet joins the one scrim (it was a separate 0.70). A guard test fails on raw `z-` numbers or black overlays in any source file.
- **P14, Drift's queued revoke says one thing.** Code decides: a Drift revoke is enqueued as `op_type: "revoke"` and the background revocation drain (`propagation.DrainRevocations`, on by default) claims it with no operator in the loop. So `ResolutionDialog` says "On its way · Revocations send on their own", neutral dot, no link, no "has not reached Zitadel yet" line. `ActionOutcome` grows `sendsItself`; the waits-for-you path (NextStep) is unchanged for everything else.

## Deviations

- No `setQueryData` on the rail indicators from `onSuccess` (spec section 5.2): this wave keeps counts honest by writing nothing ahead of the server. Left for the flow wave.
- The forecast is drawn in seats (rows). Dialogs (RemovalDialog, Drift) keep their busy button, which already breathes.
