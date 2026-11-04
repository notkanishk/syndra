# Character and soul (A-18)

## Why

Owner, 5 Nov 2026, answered "all yes" to every question in `design_handoff_syndra/reconcile-2026-10/from-design/A-18-character-and-soul.md` §6. My access's count line ("Nine roles across six rooms") counted rooms, repeated each end date and came from records; Recent changes gave up its seat on a quiet month; project cards were in API order; and the story of the name had no home after the Zitadel card was deleted.

## What changes

- **My access header (option A).** `summarySentence` is replaced by the pure `headerLine()` (`components/member/headerLine.ts`): one line from a fixed ladder (ending within 14 days, given but not on the account, a storage step, a request with staff, an approval on its way, access starting today, calm). Every number is a count of roles Zitadel holds; records only name things and supply dates. Ending, missing and on-its-way roles jump to their card (`glideTo`, 8d). Project cards are A to Z (`sortByName`).
- **Greeting.** `greeting()` moves to `lib/greeting.ts`; My access greets by time of day like Home.
- **Recent changes** always renders once read, with "Nothing has changed in the last 30 days." when empty.
- **About Syndra** in the Account sheet above Sign out, closed by default (FoldChevron + Unfold). The story is told the exact way: Syn keeps the door of "the hall"; Frigg's hall is not named. Login line and the deleted Zitadel card were checked and already match.
- **Voice.** "The keeper's voice" paragraph and the new strings are in the copy guide, section 2.
- **Ideas 1 to 7.** Term first and last day on Home's date line (`termDayLine`); greeting; About row; the Drift and revocations all-clear titles; `lift` and `bloom` on the five queues' all-clear only after the operator's own write (`useQueueCleared(..., anyOwnAct)`, fed by the query client's mutation cache); a one-shot bloom on the SyndraMark dot at a session's first paint (`sessionStorage`, cleared at sign-out); "From today..." is ladder rung 6.
- **Fix (prod check).** The person page's cleanup hint "No bundle yet: everything they hold was given directly." is said only of a person who holds something (`backend/internal/services/views.go`).

## Not in this build

Quiet hours, the 404 in voice, the first sign-in welcome. The welcome needs the backend to record a first sign-in (Q9): a later change, listed in `openspec/NEXT.md`.
