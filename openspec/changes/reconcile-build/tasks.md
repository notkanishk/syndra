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
- [x] 3.1 Backend: `role_waits` (project, role key, `marked_by`, `marked_at`) and `propagation_outbox.wait_reason` (`role_waits` | `manual` | `halted:<reason>`), migration 000056
- [x] 3.2 One rule, `services.Deliver`: cascades and bundle publish (`applyMode`), grants and removals by hand, request approval and bulk direct ops all decide through it. A row waits when a role it carries is marked; until the switch over (`confirmation.model = roles`) the old modes and `?apply=true` still hold their changes too, so nothing reaches Zitadel sooner than before. The unattended revocation runner skips rows whose role waits
- [x] 3.3 API: `GET /roles-that-wait` (with the one-time move's suggestions until the switch), `PUT|DELETE /roles-that-wait/{project}/{role}` (audited), `PUT /config/confirmation-model`; the role catalogue carries `waits_for_send`; grants, removals, approvals and cascades return `wait_reason`
- [x] 3.4 Roles that wait for you at `/changes/roles-that-wait` (`/automation/settings` redirects): the one-time move ("Mark N and finish", "Mark none"), the list by project with Marked, People and Stop waiting, Add roles through the role picker with its before-line, the waiting-changes line
- [x] 3.5 WaitsMark: picker rows and legend, a project's When given column and line, rule rows and editor, Requests (Asked for), a person's role rows, Give a role
- [x] 3.6 Nav 3a: Automatic rules and Access map at top level; a Changes group with Pending changes, Change history, Roles that wait. The drift sound moved to the Account sheet (Segmented Off · On, its three state lines)
- [x] 3.7 Words from `wait_reason` (`lib/wait-reason.ts`): Pending changes "Why it's waiting", Give a role before-line and outcomes, approval outcomes, the new role's "Make it wait for you →", the rule editor's before-line; rule mode controls removed
- [ ] 3.8 [ready-for-human] Run the live-database test `TestRoleWaits_MarkReadAndRecordAgainstTheSchema` once migration 000056 is on a test database (needs the owner's yes to migrate one)
- [ ] 3.9 [ready-for-agent] After the switch over runs on production: a migration dropping `bundles.confirmation_mode`, `mapping_rules.confirmation_mode` and the global default setting, and the code that reads them (`applyMode`'s mode, `?apply=true`)
- [ ] 3.10 [ready-for-agent] Bundle editor: When given per role and its summary sentence; publish review grouped Straight away · Waits for you · Unchanged (with step 4, Bundles v2)
- [ ] 3.11 [ready-for-agent] My access 5 (N19): a member's waiting approval in Recent changes (needs B10's member-scoped read)
- [ ] 3.12 [ready-for-agent] Next step reads per user, rule, request and bundle version (one query keyed by `source_ref`), so the status stays on the changed thing after a reload; today it shows on the outcome and in Pending changes
- [ ] 3.13 [needs-triage] Home and a person's Requests tab still show the old approval text (they call `useDecideRequest` without `approvalOutcome`)

## 4. Screens
- [x] 4.1 Bundles v2 (1c):
  - [x] /bundles is a list (Bundle · What it's for · Roles · People, attention in words under the name); /bundles/[id] is the page: header (Rename, Add people → People in Select mode with Add to a bundle armed, `give_bundle`), the violet unpublished panel (Discard changes, B6; Publish vN), tabs What it gives (card per project, Remove opens the cost inline, "New in vN" / "Leaves in vN" with Keep) · People (grouped by version, "Move these N to vX", how each got it, B7) · History, then Given to everyone who joins and Retire
  - [x] New bundle: lg, name and purpose side by side, Start from (Nothing · A person · Another bundle) as a one-time copy into the picker. B5's `copy_from` on the backend was not needed: the picker ticks what it copies, so the request already carries the roles
  - [x] Role picker (2a, 3a): chosen roles as removable chips, a fold per project with "k of n" / "n roles"
  - [x] Publish dialog: a dropped role's "−" is amber
  - [ ] 4.1a [needs-info] B8 included bundles needs the owner's yes before building (and B9's bundles in the picker with it)
  - [ ] 4.1b [ready-for-agent] The unpublished panel's "−" line could say what dropping would cost ("If people move, 9 of the 11 lose …"); today that is in the row's Remove panel
  - [x] B6 backend: `DELETE /bundles/{id}/draft` resets `bundle_roles` to the latest version's roles in one tx, audited `bundle.draft_discarded`, no outbox rows; returns the fresh draft
  - [x] B7 backend: migration 000057 adds `assigned_by`, `source`, `moved_from_version` to `user_bundle_assignments`; assign = `direct` + actor, welcome = `welcome` + nobody, publish-migrate and move = `moved` + actor + the version left. `request` is reserved: an access request asks for one role, never a bundle. Older rows stay NULL and the API omits the fields
- [ ] 4.2 Insights (N12)
- [ ] 4.3 My access (B10, 52b, 52c)
- [x] 4.4 People bulk (B4): a bulk revoke leaves the requester out (`excluded`), the bar says "Includes you · you are left out of anything that revokes access" (the copy guard replaced the board's "takes access away" with "revokes"), and the review row says "Left out · this is you"

## 5. Phone and login
- [ ] 5.1 Filters below tablet as a Modal sm sheet (87c)
- [ ] 5.2 Insights below tablet as sentences (20a)
- [ ] 5.3 Login: "Sign in again in a new tab" (21b)
