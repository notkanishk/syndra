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
- [ ] 3.9 [ready-for-agent] Once migration 000058 (the switch over, on every deployment) has run on production: a migration dropping `bundles.confirmation_mode`, `mapping_rules.confirmation_mode` and the global default setting, and the code that reads them (`applyMode`'s mode, `?apply=true`)
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
  - [x] 4.1a B8 included bundles (owner's pick 2e) and B9 bundles in the picker:
    - [x] Backend: `bundle_includes`, one level deep (409 `BUNDLE_NESTING`), includes in the working copy, `/includers`, publish asks each includer update/leave (409 `INCLUDERS_UNANSWERED`), retire refused (409 `BUNDLE_INCLUDED`), "Via {parent}, from {child}"
    - [x] Picker (3a): a "Bundles" fold first, only where `bundles` is passed (New bundle, Add roles); a chosen or included bundle's roles ticked and locked "In {bundle}"; blocked rows say why ("Includes …, so it can't go inside another bundle" / "{this} is inside …, so it can't include other bundles"); never lists itself; chips "· bundle · n roles"
    - [x] New bundle sends `includes`; footer note "{bundles} and n roles of your own · m roles in all"
    - [x] What it gives: an "Included bundle" card per include (Remove from {parent}, "Comes with {child}", "Change what's in {child} →"), "{child} vN is out … Add to next version"; include changes in the unpublished panel and the publish dialog
    - [x] An included bundle's page: Directly / Through bundles / Behind vN StatCards (StatCard gains `href`) jumping to the People groups; the dashed "Through other bundles" card with "Update to vN"
    - [x] Publish asks each including bundle Update or Leave, no default, and sends `includers`; Retire's end card says "Refused while … include it." and disables the button; a person's bundle role reads "Via {parent}, from {child}"
    - Tests: picker blocking and locking, New bundle includes, include card and Add to next version, publish includer gating, Through other bundles, retire refusal, the via line
  - [ ] 4.1b [ready-for-agent] The unpublished panel's "−" line could say what dropping would cost ("If people move, 9 of the 11 lose …"); today that is in the row's Remove panel
  - [x] B6 backend: `DELETE /bundles/{id}/draft` resets `bundle_roles` to the latest version's roles in one tx, audited `bundle.draft_discarded`, no outbox rows; returns the fresh draft
  - [x] B7 backend: migration 000057 adds `assigned_by`, `source`, `moved_from_version` to `user_bundle_assignments`; assign = `direct` + actor, welcome = `welcome` + nobody, publish-migrate and move = `moved` + actor + the version left. `request` is reserved: an access request asks for one role, never a bundle. Older rows stay NULL and the API omits the fields
- [ ] 4.2 Insights (N12)
  - [x] 4.2.1 Access map is Insights at /insights (nav leaf, title); /graph redirects
  - [x] 4.2.2 Insights reads, all built (owner: build all of it, smallest blast radius first):
    - `GET /insights/findings` (single route at ≥ 90% with at least 5 holders, overlap, unused), `GET /insights/who` (who has X without Y), `POST /insights/whatif`
    - `GET /insights/trends?period=30d|term|year`, from the record: weekly changes, requests, delivery, drift, ending soon, upkeep. Academic terms are Aug–Dec, Jan–May, Jun–Jul (owner to confirm, 4.2.6)
    - As of a date: migration 000061 `insights_snapshots`, one picture a day, taken by a periodic runner every 6 h. `GET /insights/timeline`, `GET /insights/asof?day=` (409 `NO_SNAPSHOT` with `first_day`)
    - Recent use: migration 000062 `usage_last_seen` (user, project, `sign_in` | `badge`), filled from Zitadel sign-in events on the existing event webhook. `GET /insights/usage`, `GET /users/{id}/usage`; pruned by a runner. Badge taps have no source yet (4.2.7)
    - Lenses: `GET /insights/lens?kind=ending&days=1..365` and `kind=group&group=left|new_term|team:<name>`. Counts are observed holders. "Left" is inactive, locked, deleted or departed. "New this term" is the account's creation date in Zitadel, and the first record in Syndra only when Zitadel has none
  - [x] 4.2.3 Slice I1 (UI only, existing reads): Map and Trends tabs, the link holds tab/question/view/pins/finding, lane map with Follow and pins, answer panel with Copy as sentences, drift / waiting for you to send / unused chips, keys, phone rest cards (20a). The rest (questions, findings, the at-rest glance, lenses, dates, Trends) followed in I2 to I6, below
  - [x] 4.2.4 Slice I7 (UI, on the `/insights/lens` and `/insights/usage` reads): the lens control on the Map tab (People · Recent use · Ending soon · A group; `lens=`, with `days=` 7/30/90 or `group=` left / new_term / team:<name>), each lens lighting its roles with a per-role count, a legend sentence and an answer panel; Recent use shows held-but-not-used only when sign-ins are received, else one sentence; the Held but not used card on Trends; keys L (next lens) and T (choose a date, as the board's key list says); on an earlier date the other lenses are disabled with a reason. Not drawn: the board's fade by recency (the read gives only "held and unused"), and per-edge weight for a group (the read is per role)
  - [x] 4.2.5 Trends tab and as-of timeline (UI on the 4.2.2 reads): Trends has a period segment (`period=30d|term|year`) and its cards, including Held but not used. The as-of timeline draws the map from that day's picture, hides findings, puts `asof=` in the link and offers "Back to now"
  - [ ] 4.2.6 [needs-info] Term dates: Trends and the "new this term" group use Aug–Dec, Jan–May and Jun–Jul. Confirm (Q-06 §7)
  - [ ] 4.2.7 [needs-info] Badge taps: Recent use has no source for them yet, so the badge half stays empty and says so. Connect a source, or hide the card until then (Q-06 §7)
  - [ ] 4.2.8 [needs-info] Held but not used has no "Last used" column, because Syndra keeps when a person last signed in to an app, not when they used a role. Is the card right without it? (Q-06 §7)
  - [ ] 4.2.9 [needs-info] Recent use on the map does not fade roles by recency; only roles held but not used are lit, since that is what the record shows. Enough? (Q-06 §7)
  - [ ] 4.2.10 [needs-info] The what-if answer panel names who gains and loses access, but nothing draws a thread from it back to the map. Wanted? (Q-06 §7)
  - [ ] 4.2.11 [ready-for-human] Run the gated live-DB tests against a test database (`SYNDRA_TEST_DATABASE_URL`, migrations through 000062 applied): `insights_trends_live_test.go`, `insights_snapshots_live_test.go`, `usage_live_test.go`, `insights_lens_live_test.go`, `bundle_includes_live_test.go` (`TestBundleIncludes_AgainstTheSchema`), and the request-kind tests in `requests_live_test.go` (extend requests, migration 000059)
  - [x] 4.2.12 Zitadel sends sign-in events (owner, 17 Oct 2026): executions for `oidc_session.added` and `user.token.added` on the existing `syndra-event-listener` target. `user.token.v2.added` is refused by Zitadel (`COMMAND-74aaqj8fv9`), so it is out of the manifest and the translator. Rows arrive once #46 is deployed
- [x] 4.3 My access (B10, 52b, 52c, N19)
  - [x] 4.3.1 Backend B10: `GET /me/changes?days=30` (1–90, `withUserAuth`, the caller only; the proxy allows it as a `/me` route). Items newest first `{kind, at, project_id, role_key, by?, note?, via, until?, waiting?, wait_reason?}` from `db.ListMyChanges`: requests from `access_requests` (approved, rejected, withdrawn; a fulfilled request is its grant's row); given and taken away from the Zitadel outbox rows (a revoke's audit row names no grant, so the outbox row from the same transaction is the record that still names the role), cancelled sends (superseded, abandoned) and the approval's own row left out. `by` is absent for Syndra, a rule or a lifecycle step; `via` is the bundle or rule
  - [x] 4.3.2 Recent changes card (`components/member/RecentChanges.tsx`) leads the page when the feed has anything and isn't drawn otherwise, the empty state included: dot and word, the sentence with the role in bold, the note quoted with the decider's name, the date in a 110px seat, "Ask again →" on Not approved (Ask for access prefilled), "All your requests →"
  - [x] 4.3.3 Role rows: "Ready to use" and "Checking…" gone; the second line is why you have it; only "Not there yet" (amber) and "Couldn't check" (grey) are added. The expiry is its row: warn-soft, "Given to you until {date} · N days left", "Ask to keep it longer →", sorted first in its card; the warn-note removed. Summary "{N} roles across {M} rooms. One runs out on {date}." Role names from the catalogue's labels
  - [x] 4.3.4 N19: an approval whose outbox row is still pending carries `waiting` and `wait_reason`; Recent changes shows it Approved with the amber dot and "It isn't on your account yet: …" (role_waits, or halted: "Syndra couldn't reach the sign-in system…"); its room row reads "Not there yet · approved {when}, and {reason}". A queued role with no such approval reads "Not there yet". Members never see "Waiting for you to send"
  - [x] 4.3.5 Catalogue 52b: one search; rooms as folds (28px round chevron, `Unfold`, 52px rows) counting "You have 3 of 4" · "3 you can ask for" · "You have all of it"; a search opens rooms with a match (role label, description or room name), tints the match, folds the rest with "No match"; nothing found points at Ask for access
  - [x] 4.3.6 Tests: handler and shaping (`my_changes_test.go`), the statement's shape, and a live-DB test `TestListMyChanges_AgainstTheSchema`; vitest for each row kind and both waiting variants, the expiry row first, the catalogue folds and search
  - [ ] 4.3.7 [ready-for-human] Run `TestListMyChanges_AgainstTheSchema` against a test database (skipped here: no SYNDRA_TEST_DATABASE_URL)
  - [x] 4.3.8 Decided: the copy guide's "revoked" stands, so the row says "Revoked" / "{role} was revoked." and not the board's "Taken away" (Q-06 §1)
  - [x] 4.3.9 Decided by the owner, 16 Oct 2026: "Ask to keep it longer →" is a request to extend (migration 000059, `access_requests.kind` = `access` | `extend`). Approving moves the end date of the grant they hold and never gives the role again. If the grant lapsed while the request waited, approval gives it back for the time asked. An operator's own extension uses the same rule (`handlers/request_kinds.go`). The board drawing is still asked for (Q-06 §3)
  - [ ] 4.3.10 [needs-info] Copy the board doesn't draw: Withdrawn ("Your request for {role} was withdrawn."), Given via a bundle ("…, because you're in {bundle}.") and via a rule ("{role} came with your other access, automatically."), a search with no match ("Nothing on offer matches "{q}". Ask for access, and say what you need under Why."), and the halted row reason without "when it was approved". Also: every room starts folded (the board opens one)
  - [ ] 4.3.11 [ready-for-agent] A failed `/me/changes` read draws nothing (the card gives up its seat); consider a quiet Notice
- [x] 4.4 People bulk (B4): a bulk revoke leaves the requester out (`excluded`), the bar says "Includes you · you are left out of anything that revokes access" (the copy guard replaced the board's "takes access away" with "revokes"), and the review row says "Left out · this is you"

## 5. Phone and login
- [x] 5.1 Filters below tablet: a Modal sm sheet titled Filters, ending in "Clear all" and one accent "Show {n} {noun}" with the list's live count (87c); popover unchanged from tablet up
- [x] 5.2 Insights below tablet as sentences (20a): rest cards, the answer in sentences with outline Pin and Copy, lens as a Select, chips that wrap, no rest cards on an earlier date; an earlier date is reached by link only (no board draws the control on a phone)
- [x] 5.3 Login N21 (21b): a SESSION_ENDED outcome offers "Sign in again in a new tab →" (`/auth/zitadel?then=close`) and "This tab stays as it is. Come back here when you're signed in."; `then` rides the PKCE cookie like `next`; the door opens on "You're signed in again." with no action, and middleware leaves it there
- [x] 5.4 Mobile 1 (amber provisional ring, step 1), Mobile 4 (the ledger now matches main's grabber copy), Mobile 5 (no "holders" in stacked rows), Login 3 (step 1)
