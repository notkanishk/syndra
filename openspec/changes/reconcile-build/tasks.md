# Tasks: reconcile build

Order and copy: `design_handoff_syndra/reconcile-2026-10/README.md`, `BUILD-LIST.md`, `RULES.md`. Tags per `docs/agents/triage-labels.md`.

## 1. Foundations (BUILD-LIST States 1–4, Selection, Buttons, Roles 1)
- [x] 1.1 `Notice` replaces `ErrorState`'s two looks (danger, "Try again", reference); `ListStates` keeps its API
- [x] 1.2 Offline and degraded are Notices inside `<main>`, above the page, offline first, no sticky band, no "!" glyph or solid fill; sample-data copy as drawn, server steps behind "For whoever runs the Syndra server"
- [x] 1.3 `Unfold`: a region opens to its height and pushes the page down, contents 140ms behind; closing fades first, then closes the gap (Motion 6a, 8a). Used by both system Notices and the server-steps disclosure
- [x] 1.4 Queued outcome: `NextStep` (amber dot, "Waiting for you to send →" to Pending changes, optional why-line) in place of the pill and QUEUED_GLOSS, in ActionOutcome (inline, block) and a person's role row (`NotSentYet`); label "Waiting for you to send". Automatic retries (revocations, withdrawn access) keep "Waiting to be sent"; they aren't the operator's to send
- [x] 1.5 `ActionOutcome placement="seat"` with `word` (proposed/ActionOutcome-seat.md); outcomes fade into their slot (2a)
- [x] 1.6 "Close" for every dialog with a result (#96b); FilterPanel's Done stays
- [x] 1.7 Selection seat: `Seat` in `RowCheckbox` and `SelectAllRow`; `ChoiceRow` for pickers (bundle `RolePicker` uses it); chosen list rows take full `--accent-soft`
- [x] 1.8 error.tsx is the danger Notice with a violet "Go to the home page →"; not-found.tsx is an EmptyState
- [x] 1.9 Buttons: "Reading…" becomes "Reading". The login's "Opening…" stays: the login board draws it (Login 2, as built)
- [x] 1.10 /roles heading "People"; bundle notes and labels say people
- [x] 1.11 Session-ended detail no longer repeats "Nothing was changed." (Login 3)
- [x] 1.12 Freshness: the provisional dot is an amber ring (Mobile 1)
- [ ] 1.13 [ready-for-agent] `PlanReview` rows still show "Waiting for you to send" as a plain pill; a plan row is a forecast, so it keeps the word without a link. Confirm with design whether plan rows link too
- [ ] 1.14 [ready-for-agent] People "title · team, then email" in pickers (People everywhere 1)

## 2. Never sample data (N23, B11, States 5)
- [x] 2.1 Zitadel source keeps the last good read: entries expire, never vanish; invalidation expires; a failed list read serves the last good read (held one TTL) and marks the directory not current
- [x] 2.2 With nothing ever read: `directory.ErrUnreachable`, answered 503 `DIRECTORY_UNREACHABLE` with no rows (`viewError`)
- [x] 2.3 Configured but no client: the `unreachable` source, never the demo. Demo only when `ZITADEL_DOMAIN` is unset
- [x] 2.4 `/system/mode`: `last_read_at` (null when never read); `degraded` when configured and not answering
- [x] 2.5 Degraded Notice: last known ("This is what it held at 09:40", its age) or never read; "These numbers are not real." deleted
- [x] 2.6 `ListStates` and `ErrorState` render nothing for a never-read directory
- [ ] 2.7 [ready-for-agent] PageHeader meta that counts people or roles is omitted until a read exists (per page)
- [ ] 2.8 [needs-triage] The last good read lives in memory: a restart while Zitadel is down starts at never-read. Persist it if that matters
- [ ] 2.9 [ready-for-agent] DegradedBanner.md: giving and revoking buttons disabled while degraded (check what `useDegraded` already disables)

## 3. Confirmation, option B (N14–N18)
- [ ] 3.1 Backend: `waits_for_send` on (project, role key) with `marked_by`, `marked_at`; `wait_reason` on outbox rows
- [ ] 3.2 Every path decides by the role's flag: cascade and bundle publish, grants by hand (replacing `?apply=true`), request approval
- [ ] 3.3 WaitsMark, RolePicker (one for every role choice), Roles that wait settings, project When given, nav 3a
- [ ] 3.4 Next step reads per user, rule, request and bundle version; outcomes say why a send didn't happen

## 4. Screens
- [ ] 4.1 Bundles v2 (B5–B7, B9; B8 needs the owner's yes)
- [ ] 4.2 Insights (N12)
- [ ] 4.3 My access (B10, 52b, 52c)
- [ ] 4.4 People bulk (B4: never take access from yourself)

## 5. Phone and login
- [ ] 5.1 Filters below tablet as a Modal sm sheet (87c)
- [ ] 5.2 Insights below tablet as sentences (20a)
- [ ] 5.3 Login: "Sign in again in a new tab" (21b)
