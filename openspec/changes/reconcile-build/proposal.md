# Reconcile build: what design has given, built in order

## Why
The October reconcile (`design_handoff_syndra/reconcile-2026-10/`) settled every difference between the boards and the shipped app by 14 Oct 2026. The TrueNAS slice shipped under `truenas-reconcile`. This change builds the rest, in the package's own order (README "Suggested build order"), each step shippable alone.

## What changes
1. **Foundations.** One Notice for every dysfunction, in the content column (6a). The queued outcome says the step left ("Waiting for you to send →"), never a pill. ActionOutcome gains the `seat` placement. "Close" is the way out of a dialog with a result (#96b). The round selection seat replaces the checkbox (SelectionSeat), and ChoiceRow is the picker row. Button grammar: no trailing ellipsis. Say "people", never "holders".
2. **Never sample data (N23, B11).** A Zitadel that stops answering leaves Syndra showing its last good read, marked not current. With none, reads are 503 `DIRECTORY_UNREACHABLE` and show nothing. The demo directory only serves when Zitadel isn't configured.
3. **Confirmation, option B.** The `waits_for_send` role flag, WaitsMark, RolePicker, Next step, Roles that wait, nav 3a.
4. **Screens.** Bundles v2, Insights, My access, Storage, Connected systems, People bulk.
5. **Phone and login.** Mobile 1–5, Login 1 and 3.

Motion: new parts move by the board's picks (2a, 6a, 8a) with the existing six tokens. The wider sweep waits for design's answer to Q-05 (`truenas-reconcile` E13).

## Not in this change
The access-change planner (`architecture-deepening`): deferred until this is built. Benched additions N24–N27.
