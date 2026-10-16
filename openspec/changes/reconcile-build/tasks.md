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
- [x] 1.14 People "title · team, then email" in pickers (People everywhere 1): one `personLine()` in `lib/format`, used by the Assign dialog (TargetOverview) and Start from a person (Bundles); an empty half is left out, never "—" (People's own Team column keeps its "—"). Tests

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
- [x] 3.11 My access 5 (N19): a member's waiting approval in Recent changes and in its room (see 4.3.4)
- [ ] 3.12 [ready-for-agent] Next step reads per user, rule, request and bundle version (one query keyed by `source_ref`), so the status stays on the changed thing after a reload; today it shows on the outcome and in Pending changes
- [x] 3.13 Home and a person's Requests tab decide through the queue's own `useRequestDecisions()` (`lib/request-decisions`, with `approvalOutcome()`), so every approval says the same thing per `wait_reason`, in the row's Decision seat. Home's decided row also keeps its seat now (it vanished on the tap, outcome and all, and named the requester by id). Tests

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
  - [x] 4.2.1 Access map is Insights at /insights (nav leaf, title); /graph redirects
  - [ ] 4.2.2 [needs-info] The Insights board's questions, lenses, findings, as-of-a-date and trends need new backend reads (per-person traces, graph snapshots from the audit log, overlap and unused analysis) and one new dependency (usage events from badge taps and app sign-ins). Owner to say which come first; the single-route threshold is also unsettled (drawn at ≥ 90%)
- [x] 4.3 My access (B10, 52b, 52c, N19)
  - [x] 4.3.1 Backend B10: `GET /me/changes?days=30` (1–90, `withUserAuth`, the caller only; the proxy allows it as a `/me` route). Items newest first `{kind, at, project_id, role_key, by?, note?, via, until?, waiting?, wait_reason?}` from `db.ListMyChanges`: requests from `access_requests` (approved, rejected, withdrawn; a fulfilled request is its grant's row); given and taken away from the Zitadel outbox rows (a revoke's audit row names no grant, so the outbox row from the same transaction is the record that still names the role), cancelled sends (superseded, abandoned) and the approval's own row left out. `by` is absent for Syndra, a rule or a lifecycle step; `via` is the bundle or rule
  - [x] 4.3.2 Recent changes card (`components/member/RecentChanges.tsx`) leads the page when the feed has anything and isn't drawn otherwise, the empty state included: dot and word, the sentence with the role in bold, the note quoted with the decider's name, the date in a 110px seat, "Ask again →" on Not approved (Ask for access prefilled), "All your requests →"
  - [x] 4.3.3 Role rows: "Ready to use" and "Checking…" gone; the second line is why you have it; only "Not there yet" (amber) and "Couldn't check" (grey) are added. The expiry is its row: warn-soft, "Given to you until {date} · N days left", "Ask to keep it longer →", sorted first in its card; the warn-note removed. Summary "{N} roles across {M} rooms. One runs out on {date}." Role names from the catalogue's labels
  - [x] 4.3.4 N19: an approval whose outbox row is still pending carries `waiting` and `wait_reason`; Recent changes shows it Approved with the amber dot and "It isn't on your account yet: …" (role_waits, or halted: "Syndra couldn't reach the sign-in system…"); its room row reads "Not there yet · approved {when}, and {reason}". A queued role with no such approval reads "Not there yet". Members never see "Waiting for you to send"
  - [x] 4.3.5 Catalogue 52b: one search; rooms as folds (28px round chevron, `Unfold`, 52px rows) counting "You have 3 of 4" · "3 you can ask for" · "You have all of it"; a search opens rooms with a match (role label, description or room name), tints the match, folds the rest with "No match"; nothing found points at Ask for access
  - [x] 4.3.6 Tests: handler and shaping (`my_changes_test.go`), the statement's shape, and a live-DB test `TestListMyChanges_AgainstTheSchema`; vitest for each row kind and both waiting variants, the expiry row first, the catalogue folds and search
  - [ ] 4.3.7 [ready-for-human] Run `TestListMyChanges_AgainstTheSchema` against a test database (skipped here: no SYNDRA_TEST_DATABASE_URL)
  - [ ] 4.3.8 [needs-info] "Taken away" is drawn on the board; the copy guide's one word is "revoke" and its guard refuses the other, so the row says "Revoked" / "{role} was revoked." Owner to choose
  - [ ] 4.3.9 [needs-info] "Ask to keep it longer →" opens Ask for access with the role chosen, but a request is only for a role you don't have (15 Oct): the backend answers ALREADY_HELD. Either the ask dialog offers an extension, or the link goes elsewhere
  - [ ] 4.3.10 [needs-info] Copy the board doesn't draw: Withdrawn ("Your request for {role} was withdrawn."), Given via a bundle ("…, because you're in {bundle}.") and via a rule ("{role} came with your other access, automatically."), a search with no match ("Nothing on offer matches "{q}". Ask for access, and say what you need under Why."), and the halted row reason without "when it was approved". Also: every room starts folded (the board opens one)
  - [ ] 4.3.11 [ready-for-agent] A failed `/me/changes` read draws nothing (the card gives up its seat); consider a quiet Notice
- [x] 4.4 People bulk (B4): a bulk revoke leaves the requester out (`excluded`), the bar says "Includes you · you are left out of anything that revokes access" (the copy guard replaced the board's "takes access away" with "revokes"), and the review row says "Left out · this is you"

## 5. Phone and login
- [x] 5.1 Filters below tablet: a Modal sm sheet titled Filters, ending in "Clear all" and one accent "Show {n} {noun}" with the list's live count (87c); popover unchanged from tablet up
- [ ] 5.2 [ready-for-agent] Insights below tablet as sentences (20a), with 4.2
- [x] 5.3 Login N21 (21b): a SESSION_ENDED outcome offers "Sign in again in a new tab →" (`/auth/zitadel?then=close`) and "This tab stays as it is. Come back here when you're signed in."; `then` rides the PKCE cookie like `next`; the door opens on "You're signed in again." with no action, and middleware leaves it there
- [x] 5.4 Mobile 1 (amber provisional ring, step 1), Mobile 4 (the ledger now matches main's grabber copy), Mobile 5 (no "holders" in stacked rows), Login 3 (step 1)
