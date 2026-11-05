# One time zone — design

## The rule

An instant is absolute and is stored as `timestamptz`. A **day** depends on a
zone, and in Syndra it is always the makerspace's: `Asia/Kolkata`.

| Need | Backend (`internal/clock`) | UI (`lib/time.ts`) |
|---|---|---|
| the zone | `clock.Name`, `clock.Loc` (from `TZ`, default `Asia/Kolkata`) | `MAKERSPACE_TZ` |
| now | `clock.Now()` | — |
| an instant's day | `clock.Day(t)` | `civilDay(d)`, `isSameDay(a, b)` |
| a day's start | `clock.Midnight(day)` | — |
| "until <day>" | `clock.EndOfDay(day)`: 23:59:59 of it | `endOfDayISO(ymd)` |
| "for N days" | `clock.EndAfterDays(t, n)`: 23:59:59 of the Nth day after t's | `endAfterDaysISO(n)` |
| day arithmetic | `time.Date(y, m, d+n, …, clock.Loc)` | `addDays`, `daysBetween`, `daysFromToday` |
| SQL | `(ts AT TIME ZONE $n)::date` with `clock.Name`; a day as text `$n::date` | — |

An end is stored as the last second of its day (not the next midnight) so it
still reads as its own day wherever it is shown. A migration, which has no Go
to ask, writes `'Asia/Kolkata'`.

The UI formats every date and clock with `timeZone: MAKERSPACE_TZ`, so a
browser elsewhere still reads the makerspace's day, and server and client
renders agree. Vitest sets `TZ=Asia/Kolkata` so fixtures mean the same day on
any machine.

## History (migration 000079)

| Store | Source that survives | Fix | Exact? |
|---|---|---|---|
| `insights_snapshots` | `taken_at` on every row | day = `(taken_at AT TIME ZONE 'Asia/Kolkata')::date`; two rows on one day keep the first (the table's own rule) | Exact |
| `sign_in_daily` | none in Syndra (`usage_last_seen` keeps each person's latest only); Zitadel's event log keeps every `oidc_session.added` / `user.token.added` | the migration asks (`sign_in_daily_recount`, through 2026-11-05); `handlers.RecountSignInDays` reads Zitadel's log per window (halving a full answer), applies the webhook's own rules (same events, same app match, one per person+app+moment), and swaps the totals in one transaction once 5 Nov has ended. Runs with the daily usage pass; a failure leaves everything for the next | Exact against Zitadel's log |
| Insights trends/as-of | `audit_logs.created_at` | counted on read, now `AT TIME ZONE` the named zone (was a fixed offset) | Exact, no stored buckets |
| `claim_delivery_bucket` / `_hour` | — | 5-minute and hour buckets, read only as rolling windows, never as days; hour buckets stay UTC-aligned on purpose | n/a |
| `allowances.expires_at`, `.review_date` | the picked day, encoded as UTC midnight | a value exactly at midnight UTC (to the microsecond), still ahead, on a hold in force, moves to 23:59:59 IST of that day | Exact: no other writer lands on UTC midnight |
| `direct_role_grants.expires_at` (N days) | an instant, no day | not moved: the intended day is not recoverable. New ones follow the rule | not rewritten |
| `access_requests.asked_until` | the UI sent local 23:59:59 | not moved; read through `askedEnd`, which takes the end of its IST day, so even a UTC-midnight value means the whole day | exact on read |

Why a shift would have been wrong for sign-ins: a UTC day D covers IST
[D 05:30, D+1 05:30). Relabelling it as IST D misplaces every sign-in between
00:00 and 05:30 IST (it belongs to D+1), and IST D's own first 5½ hours sit
in the UTC row for D-1. A total cannot be split, so only a recount is exact.

Edge: the first recounted day is counted in full from Zitadel, including any
sign-ins before Syndra's webhook started counting (migration 000069), so it
can read higher than it did. That is the truer number.

## Guards

- `backend/internal/repoguard/one_zone_test.go`: no `time.Local` / `.Zone()`;
  no date layout formatted without `clock.` on the line; no `.UTC()` day; no
  expiry of now + N×24h; no `AT TIME ZONE 'UTC'`, `CURRENT_DATE`,
  `date_trunc('day'…)` without a zone, a timestamp `::date` without
  `AT TIME ZONE`, or a date cast to `timestamptz` — in Go and in migrations
  from 000079. Mutation-checked.
- `ui/src/lib/__tests__/time.test.ts`: no picker value through
  `toISOString()`, no `new Date("YYYY-MM-DD")`, no `` new Date(`${day}T…`) ``,
  no `toISOString().slice(0, 10)`. Mutation-checked.
