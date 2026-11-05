# Tasks: character and soul

## 1. My access
- [x] 1.1 `headerLine()` pure ladder with tone, text and link; tested rung by rung, including order, "14 days" bound and no counts from records
- [x] 1.2 Render in `--muted` with StatusDot and Crossfade; jump button (glideTo) or ArrowLink; invisible stand-in before mount
- [x] 1.3 Project cards sorted with `sortByName`; card title carries `id="card-<project>"` for the jump
- [x] 1.4 Greeting by time of day via shared `lib/greeting.ts` (Home uses it too)
- [x] 1.5 Recent changes always renders, with an empty seat
- [x] 1.6 Old count-line tests replaced

## 2. About Syndra
- [x] 2.1 Account sheet row above Sign out, closed by default, FoldChevron + Unfold, the exact story
- [x] 2.2 Login line and Zitadel page checked (no change needed)

## 3. Character
- [x] 3.1 Term first and last day on Home's date line
- [x] 3.2 Drift and revocations all-clear titles
- [x] 3.3 `lift` + `bloom` on Drift, revocations, Requests, Expiring access and Holds due after the operator's own act
- [x] 3.4 One-shot bloom on the SyndraMark dot at the session's first paint (reduced motion removes it)
- [x] 3.5 Copy guide: "The keeper's voice" and the new strings

## 4. Backend fix
- [x] 4.1 No-bundle hint only when the person holds something; test added

## 5. Later
- [x] 5.1 Record a first sign-in in the backend, then the one-time welcome (A-18 idea 8; owner 5 Nov, Q9 yes)
  - Migration `000075_first_sign_in`: `person_first_sign_in (user_id PK, first_sign_in_at)`, written by `RecordFirstSignIn` with `INSERT ... ON CONFLICT DO NOTHING`, so it is written once and never overwritten.
  - `POST /me/first-sign-in` (the OIDC callback calls it once per sign-in) answers `first_visit`, true only for the call that wrote the row. The callback puts that in the session cookie as `firstVisit`, so the welcome shows for that session only, survives a refresh, and never returns. Any failure reads as "not the first visit".
  - Backfill: Syndra kept no record of its own sign-ins (sessions are cookies), so everyone it holds a record of is marked from their earliest one (audit-log actor, access request, sign-in event in `usage_last_seen`, grant in `zitadel_grants_index`). Nobody who has used Syndra is welcomed. People with no record at all have never touched Syndra and still get the welcome.
  - UI: "Welcome to Syndra. It keeps the makerspace's list of who can use what, and why." in My access's line seat (above every rung) and on Home under the greeting, first visit only.
  - Tests: live-DB (written once, never overwritten, first only for the recording call, the backfill); guard mutation-checked; session cookie round trip; Home and My access render.
- [x] 5.2 Quiet hours (idea 9; owner 5 Nov): Home's calm guidance gains "It will keep until morning." from 22:00 to 05:00 on a calm day only; `greeting()` reads Evening in those hours. `isQuietHour` in `lib/greeting.ts`, boundaries tested.
- [ ] 5.3 The 404 in voice (idea 10): not building

- [x] 5.4 Since you were last here (A-20 P18; owner 5 Nov, Q9 yes): migration `000076_last_sign_in` keeps the previous and current sign-in beside the first, `RecordSignIn` moves them on each sign-in, `previous_sign_in_at` rides `POST /me/first-sign-in` into the session. Home and My access say "since Tuesday" in their existing seats. Detail in `changes/second-pass/tasks.md` (P18).
