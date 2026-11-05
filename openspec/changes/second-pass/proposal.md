# The second pass (A-20)

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md`. Owner said "all yes" on 5 Nov 2026, so the first wave (P1 to P8) is built. Each part adds its own section below; the other parts' sections arrive on merge.

## P5 P8

**P5, one word per state across the System group.** Zitadel, Connected systems, a system's page, service accounts and the member's storage card say one word per state, from one table (`SYSTEM_WORDS` in `components/targets/TargetReading.tsx`): Answering, Not answering, Offline, Waiting to retry, Not connected.

- Zitadel's seat and Notice use that table. One or two missed polls read "Not answering" with the check pips and raise no Notice; three in a row read "Offline" with a neutral Notice titled "Offline." and Syndra keeps checking. "Unreachable" is gone. A Check again sits beside the line. The page header never says a count it has not read (no "0 projects, last known" while projects load).
- A member reads "Answering", not "Online".
- A system's "Waiting on you" keeps one card that does not come and go: a skeleton while the reads are out, "Nothing is waiting on you." when every read answered and nothing is, nothing when a read failed.
- Copy: "Not created by Syndra" everywhere; "by hand" for "direct grant"; "Cancel the change" for a service account's queued item; full stops on five EmptyState titles; the connected-systems empty state names the step, not a file.
- Not in this change: P11 (the Technical detail disclosure, raw ids), which is the third wave.

**P8, the forecast reaches every act.** `ActionForecast` and `FORECAST` now cover Drift (single and bulk), Remove (direct and bundle), Give, bulk apply (`RehearsalDialog`) and Cancel a queued change, beside Approve, Extend and Lift which had it.

- `forecastFor(act, roleWaits)` returns "Will wait for you to send" for an act that reaches Zitadel (approve, extend, remove, give) on a role that is one of Roles that wait. The mark is the same one the role catalogue, the plan row and the roles-that-wait list carry. A bulk apply reads the plan's own `queued` rows instead.
- `FORECAST` gains give, cancel, adopt and mark; the copy guide's section 5 table and `forecastGuide.test.ts` hold them to each other.
- The rail's count changes when the answer arrives: `settleIndicators` writes the `["governance","indicators"]` cache in each mutation's `onSuccess` (request decided, Drift resolved, hold lifted, role given, queued change cancelled or withdrawn) and the existing invalidation confirms it. Never ahead of the server; an error writes nothing.
- Only the true word blooms; an error shows the honest error with the forecast gone.

No backend change.
