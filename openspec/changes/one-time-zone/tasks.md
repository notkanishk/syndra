# One time zone — tasks

## 1. One zone

- [x] 1.1 `backend/internal/clock`: `Name`, `Loc` (from `TZ`, default `Asia/Kolkata`, tzdata embedded), `Now`, `Day`, `Midnight`, `EndOfDay`, `EndAfterDays`; tests incl. 23:00 UTC → next IST day
- [x] 1.2 `ui/src/lib/time.ts`: `MAKERSPACE_TZ`, `civilDay`, `isSameDay`, `addDays`, `daysBetween`, `daysFromToday`, `endOfDayISO`, `endAfterDaysISO`; `components/insights/days.ts` delegates to it
- [x] 1.3 compose defaults `TZ` to `Asia/Kolkata` (was UTC)
- [x] 1.4 Vitest runs in `Asia/Kolkata`

## 2. Days

- [x] 2.1 `db/usage.go`: totals keyed by `clock.Day`; `SignInsSince` starts at the IST midnight (was the session's)
- [x] 2.2 `db/insights_trends.go` `AuditDayCounts`: `AT TIME ZONE clock.Name` (was a fixed offset); services drop the offset; `trendNow = clock.Now`
- [x] 2.3 `cache/compiler.go` `role_ends`, `services/role_members.go` since/expires, `services/cascade.go`, `services/bulk.go` `formatDay`, handler sentences (`access`, `bundles`, `requests_bulk`, `request_kinds`): IST dates
- [x] 2.4 `handlers/action_key_rotate.go` quiet window read in the zone (no longer depends on `TZ` being set); `handlers/terms.go` reads terms at `clock.Now()`
- [x] 2.5 UI: every formatter in `lib/format.ts` takes `timeZone: MAKERSPACE_TZ`; "today" via `isSameDay`; `daysUntil` and `expiryLabel` count the same calendar days; `term.ts` / `term-dates.ts` on makerspace days; `AppSignInStrip` keys on makerspace days; audit CSV name; dead UTC-keyed `groupByDay` deleted

## 3. End dates

- [x] 3.1 Backend "N days": `handlers/access.go`, `handlers/bulk.go`, `handlers/request_kinds.go` `endAfter`, `services/bulk.go` (consequence, `lengthens`, `expiryPhrase`), `seed/demo.go` → `clock.EndAfterDays`
- [x] 3.2 `askedEnd` normalises any instant to the end of its IST day; the "has passed" checks use it, so an ask covers its whole last day
- [x] 3.3 UI pickers: `HoldDialog`, `RevokeDialog`, `HomeLimit` → `endOfDayISO`; `GrantDirectAccess` and `AskForAccess` → `endAfterDaysISO` / `endOfDayISO`, with days counted as calendar days; `format.ts` `askLengthForOperator`, `term.ts` `keepOptions`

## 4. History — migration 000079

- [x] 4.1 `insights_snapshots` re-keyed from `taken_at`, first picture of a day kept; idempotent (live test: 23:00 UTC → next day, twice)
- [x] 4.2 `sign_in_daily_recount` asked through 2026-11-05; `handlers.RecountSignInDays` recounts from Zitadel's log on the daily usage pass (tests: IST day, dedupe, waits, failure leaves all, halving)
- [x] 4.3 future UTC-midnight hold ends and review dates → 23:59:59 IST of that day (live test: moves once; lifted, past and counted ends stay)
- [ ] 4.4 Run on staging against production's copy before production; check the `[FINDINGS] sign-in totals … recounted` line after 6 Nov. `ready-for-human` — UNVERIFIED that staging's Zitadel credentials read production's event log
- [ ] 4.5 Production counts before deploy (no production access here). `ready-for-human`:
  ```sql
  SELECT count(*) FROM allowances WHERE lifted_at IS NULL AND
    ((expires_at > now() AND extract(epoch FROM expires_at) = floor(extract(epoch FROM expires_at)/86400)*86400) OR
     (review_date > now() AND extract(epoch FROM review_date) = floor(extract(epoch FROM review_date)/86400)*86400));
  SELECT count(*) FROM direct_role_grants WHERE expires_at > now();   -- not rewritten, N-days instants
  SELECT count(*), min(day), max(day) FROM sign_in_daily WHERE day <= '2026-11-05';
  SELECT count(*) FROM insights_snapshots WHERE day <> (taken_at AT TIME ZONE 'Asia/Kolkata')::date;
  ```
  Dev box copy (`syndradb`): 0 allowances, 3 future grant ends, 10 sign-in totals (3–5 Nov, 220 sign-ins), 18 snapshots of which 1 moves (19 Oct taken 05:28 IST 20 Oct, and 20 Oct's later picture is dropped).

## 5. Guards

- [x] 5.1 `repoguard` `TestDaysAreCountedInTheMakerspaceZone` (mutation-checked: UTC day format, `::date` in the session zone, N×24h expiry, `date_trunc('day')` in a migration)
- [x] 5.2 UI `time.test.ts` guard (mutation-checked: picker value through `toISOString()`)

## Open

- [ ] Other "today" sites outside W1c still read the browser's day (`member/RecentChanges.tsx`, `member/MemberAccess.tsx`, `targets/UnconfirmedChanges.tsx`, `apps/claim-words.ts`, `lib/drift-filters.ts`, `app/operations/page.tsx`, `lib/greeting.ts`). Right in an IST browser. `ready-for-agent`
- [ ] Hold, revoke and storage-limit endpoints take the instant verbatim; normalising them server-side like `askedEnd` would make an old client harmless too. `ready-for-agent`
