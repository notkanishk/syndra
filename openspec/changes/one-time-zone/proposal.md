# One time zone

## Why

Owner, 5 Nov 2026: "why are the older signins grouped to utc, timezone can just
be offseted. it still is absolute time. please fix. get everything in one IST
time zone."

Instants were always stored as `timestamptz`, so they never moved. Days did.
Until every container ran `TZ=Asia/Kolkata` (PR #102, 5 Nov 2026), several
things counted or named a day in UTC: the daily sign-in totals, the Insights
snapshots, a role's end date in claims, the dates in server-written sentences,
and the UI's audit CSV name. A sign-in at 23:00 UTC is 04:30 the next morning
in India, and it was counted on the day before.

The architecture audit (family 7, wave W1c) found the same fault in end dates.
The hold and revoke dialogs sent a picked day as UTC midnight, so "until
30 Nov" lifted at 05:30 IST on 30 Nov, while the storage-limit dialog's
"30 Nov" meant the whole day. "N days" was now + N×24h in about eight places.

## What changes

- **One zone, one helper each side.** `backend/internal/clock` (Go and SQL)
  and `ui/src/lib/time.ts` (UI). The zone comes from `TZ`, defaulting to
  `Asia/Kolkata`; compose now defaults `TZ` to it too. SQL names the zone
  (`AT TIME ZONE $n`), never the session's (the cluster's is UTC).
- **One end-date rule.** "Until <date>" lasts through the whole of that day in
  IST, stored as its last second. "N days" is N calendar days, ending as the
  Nth does. Every caller goes through `clock.EndOfDay` / `clock.EndAfterDays`
  and `endOfDayISO` / `endAfterDaysISO`.
- **History re-keyed (migration 000079).** Snapshots re-derived exactly from
  `taken_at`. Daily sign-in totals recounted exactly from Zitadel's own event
  log by the backend (SQL cannot: the sign-ins themselves are not kept).
  Future hold end and review dates that are exactly UTC midnight moved to the
  end of their day.
- **Guards.** `repoguard` `TestDaysAreCountedInTheMakerspaceZone` (Go and SQL,
  and migrations from 000079) and `ui/src/lib/__tests__/time.test.ts` (no
  picker value through `toISOString()`, no UTC day of an instant).

## Impact

- Backend: `internal/clock` (new), `db/usage.go`, `db/insights_trends.go`,
  `services/insights_{trends,asof}.go`, `services/{bulk,role_members,cascade}.go`,
  `handlers/{access,bulk,request_kinds,requests_bulk,bundles,terms,sign_ins,action_key_rotate}.go`,
  `cache/compiler.go`, `seed/demo.go`, `cmd/api/main.go`.
- UI: `lib/time.ts` (new), `lib/{format,term,term-dates,audit-vocabulary}.ts`,
  `components/insights/days.ts`, the hold, revoke, storage-limit, grant and
  ask dialogs, `AppSignInStrip`, the audit page. Vitest runs in `Asia/Kolkata`.
- Behaviour changes on purpose: ends move to day ends; a role's end date in
  claims (`role_ends`) is the IST date of the end.
