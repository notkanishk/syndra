# Build list for Claude Code (from the October reconcile)

Each item names the board it is drawn on. Boards render the real DS components; build from them.

## Backend, in one place
Every server change the boards depend on. The frontend items below refer back here.

**Storage**
- B1 `POST /me/targets/{t}/credential` returns `409 TARGET_PAUSED` / `503 TARGET_OFFLINE` instead of dispatching (Storage 1).
- B2 `GET /me/targets` items gain `reachable_since` and `checked_at`; "offline" uses the operator page's confirmed-failure threshold (Storage 2).

**Connected systems**
- B3 Everything in `proposed/Mapping-implementation-notes.md`: one Zitadel project per system, groups listed by the add-on, settings from its schema.

**People**
- B4 `POST /grants/bulk/rehearse` (and apply) drops the requester from `remove_role` / `remove_bundle` and returns `excluded: [{ user_id, reason: "self" }]` (People 1).

**Bundles**
- B5 `POST /bundles` accepts `copy_from: { bundle_id } | { user_id }`, resolved to roles at creation. Nothing links back (Bundles 4).
- B6 `DELETE /bundles/{id}/draft` discards the working copy ("Discard changes" in the unpublished panel). Nobody's access is touched.
- B7 `GET /bundles/{id}/holders` gains how each person got it: `assigned_by` (user id), `source` (`direct` | `request` | `moved` | `welcome`), and `moved_from_version` when moved. The People tab's "Given by Kanishk Singh", "Approved request", "Moved from v2" come from this.
- B8 **Included bundles** (new model, owner yes needed before building). Table `bundle_includes(parent_bundle_id, child_bundle_id, child_version)`.
  - One level deep: refuse including a bundle that has includes, and adding includes to a bundle that is included (`409 BUNDLE_NESTING`).
  - A parent version's effective roles = its own roles ∪ the pinned child version's roles. A role present both ways is held once and survives either being removed.
  - Includes live in the working copy like roles: `PUT /bundles/{parent}/draft/includes/{child}` `{ version }`, `DELETE …/includes/{child}`. The draft diff reports include changes.
  - `GET /bundles/{id}/includers` → `[{ bundle_id, name, included_version, people }]`, for the dashed "Through other bundles" card and the three stats (directly, through bundles, behind the latest).
  - `POST /bundles/{child}/publish` takes `includers: [{ bundle_id, action: "update" | "leave" }]`, required for every includer, no default. `update` publishes that parent's next version with the new child version. Its people move if `migrate` is true. The rehearsed plan covers the child's own people and every updated parent's people, each row saying which bundle it comes through.
  - "Update to vN" from the child's page and "Add to next version" from the parent's page both write the parent's draft include version (the parent's publish moves people, rehearsed as usual).
  - Retire: `409 BUNDLE_INCLUDED` with the includer list while any bundle includes it.
  - `GET /users/{id}/access` role reasons gain `via_bundle: { parent, child }` so a person's page reads "Via New members, from Safety basics".
- B9 The role picker searches bundles too: `GET /bundles` items carry `latest_version`, `role_count`, `has_includes` (to show "can't go inside another bundle").

**Directory (N23)**
- B11 No demo fallback when Zitadel is configured. Today `directory.Default` falls back to the demo directory when Zitadel is unreachable and `/system/mode` reports `degraded: true`. Instead: keep the last good directory read (people, grants, projects) with its `read_at`; serve it with `current: false` while Zitadel is down; with no snapshot, reads return `503 DIRECTORY_UNREACHABLE` and no rows. `/system/mode` gains `last_read_at` (null when never read). The demo directory is only ever used when Zitadel is not configured (local development).

**My access (member)**
- B10 `GET /me/changes?days=30`, member-scoped (the audit log stays operator-only, #37). Items, newest first: `{ kind: "request_approved" | "request_declined" | "request_withdrawn" | "given" | "taken_away", at, project_id, role_key, by (user id, absent when automatic), note (the decider's reason or the grant's reason), via: { bundle_id } | { rule_id } | null, until }`. Returned whether or not the member holds anything now.

## My access (route-me; 52b + 52c)
1. **Recent changes card leads the page** when the feed has anything; otherwise it is not rendered. Row: StatusDot + word (Approved, Not approved, Given, Taken away, Withdrawn), the sentence with the role in bold, the note quoted with the decider's name, the date in a 110px right seat. "Not approved" carries "Ask again →" (Ask for access prefilled). Header link "All your requests →". Shown in the empty state too.
2. **Role rows drop "Ready to use".** The second line is why you have it. Only "Not there yet" (amber) and "Couldn't check" (grey) are added, from the same standing logic.
3. **Expiry lives on its row:** warn-soft tint, "Given to you until {date} · N days left", "Ask to keep it longer →"; that row sorts first in its card. Remove the separate warn-note and its ButtonLink.
4. **Catalogue (52b):** one search, rooms as folds ("You have 3 of 4", "3 you can ask for", "You have all of it"); a search opens rooms with a match, tints the match, folds the rest with "No match". Summary line: "{N} roles across {M} rooms. One runs out on {date}."

## /storage: the member's page (route-storage)

1. **Refuse password changes while TrueNAS is paused or offline** (owner, 9 Oct).
   - When `lifecycle` is `draining` or `read_only`, or `reachable` is false, render no password field. This covers the first password too.
   - In its place, one sentence:
     - Paused: "You can't change your password while account changes are paused. Your current one keeps working, and you can change it once the pause lifts."
     - Offline: "You can't change your password while TrueNAS is offline. Your current one works again as soon as it's back."
   - Remove "Save it for when changes start again", and the Paused notice's closing paragraph that promises the password will be applied later. Syndra keeps no copy, so nothing could apply it.
   - Backend: `POST /me/targets/{t}/credential` returns `409 TARGET_PAUSED` / `503 TARGET_OFFLINE` instead of dispatching.
2. **Status beside the system's name.**
   - Green: "Online and accepting sign-ins · checked a minute ago". Before the account can sign in (not created yet, or no password yet): "Online · checked …".
   - Amber: "Offline since 09:40".
   - Add `reachable_since` (when it stopped answering) and `checked_at` to each `GET /me/targets` item.
   - Show "Offline" only once Syndra is sure, after the same confirmed-failure threshold the operator's Connected systems page uses. One slow answer stays green.
   - If Syndra can't tell, show nothing rather than guess.
3. **Offline Notice comes first in the card** (amber, `role="alert"`). This is the one state where the files really are out of reach.
   - Title: "TrueNAS is offline"
   - Body: "It stopped answering at 09:40, so your shares won't open until it's back. Your account and files are safe, and your access hasn't changed. Makerspace staff can see this too, and this page updates on its own when TrueNAS returns."
   - The page refetches on its own and needs no Try again.
4. **One block per share.** Replace the three lists (Your shares, You are using, Connecting) with one block per share: its name, its usage, the bar, then the Mac/Linux and Windows addresses, each a `CopyableValue`.
5. **Usage bar is neutral** (`--muted` on `--line`), amber with "nearly full" from 90%. No quota reads "no limit set". Offline reads "Usage unavailable while offline", never 0.
6. **The password is the next step.** When the account exists without a password (`storage.needs_password`) or needs re-enrolment:
   - The form moves directly under the state line, in the violet step panel (accent-soft, accent-line). "Set password" is the page's only accent button.
   - Above the shares, one accent-edged line: "Set your password to open these · Until then, TrueNAS turns every sign-in away · Set it above ↑".
   - This replaces the amber "not switched on yet" box. It is a step to take, not a fault.
7. **When a password exists, changing it is a quiet section at the end**: outline "Change password", with when it was last changed.
8. **Each state opens with one display line** under the step line: Your account is ready / Set a password to switch your account on / Your account is on its way / None of your roles reaches TrueNAS. The copy is on the board's frames.
9. **Connection guide** (4b; see `proposed/ConnectionGuide.md`). A ghost sm button, "First time? How to connect", opens a Modal. The device is detected from the user agent: Mac / Windows / Linux / iPhone/iPad / Android. The share comes from what the member's access reaches, and the host from `view.connection.host`. Hide the button while the system is offline.
10. **One Notice per card.** For a paused system, the Notice comes after the account and shares. For a system that is paused while the account is still being created, it comes inside the waiting state.

## Connected systems (route-system-targets-id, route-system-targets-id-mappings, route-systems)
See `proposed/Mapping-implementation-notes.md`. It covers: one Zitadel project per system, set before any write; the project step (3a) and the setup strip; mapping as role → group, with groups listed by the add-on and settings from its schema; the project shown read-only on Connected systems.
1. **Assign an account to a person (#57)** in `TargetOverview.tsx` `AdoptPanel`: becomes a Modal titled "Assign {account} to a person". Replace the Syndra-ID Input with a people search (name or email) listing matches as single-choice rows (selection seat with a dot, avatar, name, then "title · team", then email), noting "Has a {target} account already". Below it, a two-row Changes / Stays box. Keep ConfirmByTyping and the accent button "Assign {account} to {name}", plus Cancel (ghost). Frontend only: people search already exists.
   - A person who already has an account on the target can't be chosen; their row reads "Already has {username} on {target}".

## People everywhere (pickers and rows)
1. **Who someone is reads "title · team", then email.** Team is Zitadel user metadata (`team`), a free-text label with no access meaning. Leave it out when empty, never "—" in a picker. People's own table keeps "—" in its Team seat.

## People (route-users)
1. **Never take access from yourself in bulk** (#32, either option). When a bulk plan that takes access away (`remove_role`, `remove_bundle`) includes `session.id`, the plan leaves that person out. The bar's `composition` reads "Includes you · you are left out of anything that takes access away". Backend: `POST /grants/bulk/rehearse` drops the requester from removal ops and returns them as `excluded: [{ user_id, reason: "self" }]`. Owner picked 32a (9 Oct): all five verbs stay. In the review, the operator's row reads "Left out · this is you. Change your own access from your page."

## Bundles (route-bundles)
1. **Publish dialog:** the "−" for a dropped role is `text-warn-text`, not `text-danger-text` (`PublishVersionDialog.tsx`). Everything else is as built.
2. **/bundles becomes a list, /bundles/[id] a page** (1c). The list: Bundle · What it's for · Roles · People, with status words under the name. The page: PageHeader (eyebrow Bundles, meta version · roles · people, actions Rename + Add people → People in Select mode with Add to a bundle armed), the unpublished-changes panel, Tabs, then a Card per project. Default-for-new-members and Retire move to a card at the end.
3. **Role picker** (`RolePicker.tsx`, 2a + 3a): one search over roles and bundles; chosen as removable chips; the catalog as folds per project ("1 of 4" / "3 roles"); rows use the selection seat (proposed/SelectionSeat.md). Used by New bundle and Add roles. New bundle's dialog is `lg`: Name and What it's for side by side, Start from as a Segmented (Nothing · A person · Another bundle).
4. **Start from** copies once. Backend: `POST /bundles` accepts `copy_from: { bundle_id } | { user_id }`, resolved to roles at creation; nothing links back.
5. **Included bundles (new model; backend).** `bundle_includes(parent_id, child_id, child_version)`, one level deep: a bundle with includes can't be included, and a bundle that is included can't include others. The effective roles of a parent version are its own plus the pinned child version's. A child publish never changes a parent: the parent's page offers "Add to next version", which goes into its working copy. The child's Publish dialog asks per parent bundle, Update or Leave, with no default; Update publishes that parent's next version, and its people move the way the people question was answered. Retire is refused while any bundle includes it. On a person's page the source reads "Via {parent}, from {child}".
6. **Bundle People tab:** grouped by version, with Move these N to vX on older groups; for an included bundle, a dashed "Through other bundles" card (square tiles, the version each includes, people, Update to vX), and three StatCards linking to the groups.

## Selection (everywhere)
1. **The round seat replaces the checkbox** in RowCheckbox and SelectAllRow, and in the new ChoiceRow. See proposed/SelectionSeat.md. Chosen list rows take `--accent-soft`, full width.

## Roles (route-roles)
1. **Column heading "Holders" becomes "People"** on /roles and anywhere else the count of people holding a role is headed (#47).

## Buttons everywhere (N4)
- No trailing ellipsis on any button.
- Accent marks the one primary action per region.
- Outline for row and secondary actions.
- Danger-outline "Revoke" for anything that takes access away.
- Ghost for Cancel.
- Refresh actions are quiet outlines.

## Phone (below tablet) on /storage
- The three-step line becomes a three-segment bar with one line under it ("Step 3 of 3 · you set a password"), the same bar as the project step.
- The status beside the system name wraps; its dot stays on the first line.
- "First time? How to connect" moves below the shares as a full-width outline button.
- The password field and its button wrap, so the field is never narrower than 220px.
- In the guide, the device switch becomes a Select, because five options don't fit a pill row at 390px.


## Insights (route-insights, replaces /graph; N12)

Nav: rename "Access map" to **Insights** (route /insights, /graph redirects; ?tab=trends). Map tab opens first. The link holds all state (question, date, lens, finding, pins).

Backend, by feature:
- Follow, remove what-if: existing graph read.
- Path between two things, Compare two people, Who has X without Y: per-person trace (every route, with source, version, end date). Who: a set query over traces.
- What if for any change: bundle review endpoint, read-only, callable from Insights with a hypothetical change.
- As of a date: graph snapshot at a timestamp from the audit log, plus a per-day change count for the timeline ticks, plus a diff (added since / counts then).
- Drift on the map: existing drift rows, keyed to role.
- Waiting for you to send on lines: pending changes keyed to edge.
- Single route: computed share of a role's people reached through one source (threshold to settle; drawn at ≥ 90%).
- Unused: roles nobody has, bundles that give nothing new (all members already hold every role another way), projects with no holders.
- Overlap: pairs of bundles sharing ≥ 2 roles, with shared and differing sets.
- Recent use lens + Held but not used: usage events from Badge reader taps and app sign-ins, last-used per person per role (90-day window). New dependency.
- Ending soon lens: end dates per edge (exists).
- A group lens: group definitions: people who left, new this term, a team; per-edge counts for that group.
- TrueNAS lane: mappings role → TrueNAS group, and connection state for the lane heading.
- Pins, keys, copy as sentences: front end only.
- Trends: weekly given/taken-away counts by route; request counts, decline reasons grouped, median time to decide; median approval-to-sent; drift found/cleared medians; ending soon by project at 30/60/90; delivery outcomes per week per system; requests for roles outside any bundle.

## Automation settings (route-automation-settings; #74, N13)
1. **Confirmation default is one control.** Replace the Segmented and the two mirrored cards with two single-choice rows (ChoiceRow, seat with a dot): "Wait for review" and "Apply at once", each with its cost sentence. Choosing saves; the ActionOutcome stays under the rows. CardHeader `note="For the whole makerspace"`; remove the warn-note and its "!" glyph. The lede under the header: "What a new rule does when its condition is met. Rules that exist already keep their own mode."
2. **The sound uses Segmented** (`size="sm"`, Off · On) in `ChimeToggle.tsx`, replacing the accent-dense pill. CardHeader title "Sound when drift is found", `note="For this browser only"`.
3. **One state line under it, only when on:**
   - Reduced motion: neutral dot, "Silent in this browser. It asks for less motion, and a sound that arrives on its own is movement in the room. Nothing will play while that is set." No button.
   - AudioContext `state === "suspended"` (no gesture yet): amber dot, "Can't play yet. This browser allows sound only after a click on the page. Play it once to allow it for this visit."
   - Otherwise: healthy dot, "Plays here. This is the tone you'll hear."
   - Outline sm "Play it once" in the last two: plays the tone through the same `playDriftChime` path, ignoring the rise check, and resumes the context. Remove "That was this." and `CHIME_FIRST_PLAY_EVENT`'s note.

## Confirmation (route-automation-settings-v5, with A in full on v2; N14–N17)

One role picker (N17): extract the bundle picker into a RolePicker used by Automation settings (Add roles), New bundle, Add to bundle, the rule editor (single choice, `mode="one"`), and Give a role (person page and People bulk). Props: chosen, locked {id → reason}, single, onConfirm. Each row shows "Waits for you" when the role's `waits_for_send`. The picker's legend line links to Automation settings, except on Automation settings itself.
Both:
- One control for this choice everywhere: Segmented "Wait for you to send" · "Apply at once", one line under it that explains the chosen option with a real example. Replaces Automation settings 1 above.
- Who set it: the config read returns `set_by` and `set_at` (dbSetConfigSetting already records the actor). Line: "Set to … by {name} on {date}." Never set: "Never set. New bundles and rules apply at once until you choose."
- Changing a mode is a setting, not a version: no plan, recorded in the audit log, changes already waiting stay waiting.

If A:
- Bundle list and detail expose `confirmation_mode`. Detail gets "How its changes reach people" (CardHeader note "This bundle only") with the control; Bundles list gets the amber "Applies at once" badge rules use. Uses the existing bulk endpoint with `kind: "bundle"`; a single-bundle PUT would be tidier.
- Settings lists every bundle and rule gathered by mode (Apply at once first), name links to its page; counts by mode in one read.
- After a default change, the outcome offers "Also switch those N" when N ≤ 3, with the list before it applies.

If B:
- Backend: a role-level flag `waits_for_send` keyed by (project, role key), with `marked_by`, `marked_at`. Every path checks it: cascade and bundle publish (`applyMode` partitions by the target role's flag instead of the bundle's or rule's mode), grants by hand (drain unless the role waits, replacing `?apply=true`), and request approval (DrainOne unless the role waits). N15.
- Every outcome that meant to send and didn't says why, warn tone, with "Waiting for you to send →": the drain's Halted reason (zitadel_offline → "Zitadel didn't answer", drain_in_progress → "another send was running").
- Remove the global default and the per-bundle and per-rule `confirmation_mode` from the UI; keep the columns until the move runs, then drop.
- Settings: "Roles that wait for you", gathered by project, with Add a role (role search), Stop waiting, and the read line linking to waiting changes for these roles.
- Project page: each role row discloses the same control; a waiting role wears a hollow neutral "Waits for you" badge.
- Publish review and rule rows name waiting roles ("goes to Pending changes", "Gives a role that waits for you").
- One-time move: suggest every role given by a waiting bundle or rule today, each with why; selection seats; "Mark N and switch over".

Copy for B, as drawn on v3:
- Settings card "Roles that always wait for you" (note "For the whole makerspace"), lede "When someone would gain or lose one of these roles, however it happens, the change waits in Pending changes until you send it. Every other role goes straight to Zitadel." Each row: role, how people get it ("Through Staff and by hand"), "Marked" who + date, people, "Stop waiting".
- Add a role: role search; before marking, "From now on, anyone gaining or losing {role} waits for you. {n} people have it today, and nothing changes for them now."
- Read-only everywhere but Automation settings (N16): a "When given" column (dot + "Straight away" / "Waits for you") on a project's roles and in the bundle editor; one line + "Set in Automation settings →" under each. No toggle, no badge on rules.
- New role dialog: neutral line before adding ("When it's given, it goes straight to Zitadel… mark it on Automation settings once it's added"); outcome offers "Make it wait for you →" (opens Add a role with it found).
- Rule editor: remove "When this rule applies". After the second role is picked: who it reaches by name, and a before-line (warn-soft if the role waits).
- Bundle editor: When given per role and one summary sentence ("When someone joins …, N reach them straight away. X waits for you to send.").
- Give a role by hand: note in the dialog before giving (warn-soft when the role waits; neutral otherwise); ActionOutcome queued "Given in Syndra. Waiting for you to send" or applied "{Name} can use {thing} now".
- Request approval outcomes: "Approved. {Name} can use it now." · "Approved. This role always waits for you, so it's in Pending changes." · "Approved. Not in Zitadel yet: Zitadel didn't answer at {time}. It goes when you send it." The last two with "Waiting for you to send →".
- Publish review grouped "Straight away" · "Waits for you" · "Unchanged", the waiting batch named after the version.
- Pending changes gains a "Why it's waiting" column: "{Role} always waits for you" or "Zitadel didn't answer when it was approved" (warn). Needs `wait_reason` on each outbox row: `role_waits` | `halted:<reason>` | `manual` (legacy).
- First visit after the update: one-time card suggesting roles given by anything that waited, selection seats, "Mark N and finish".

## Next step (proposed/NextStep.md; N16, applies product-wide)
- Every write that leaves rows in the outbox returns them with `wait_reason`; the client keeps a Next step line on the changed thing until those rows are sent (person role row, rule row, request row, bundle header).
- Reads that feed it: per user, per rule, per request and per bundle version: "outbox rows not yet sent, with wait_reason". One query keyed by source_ref.
- Before-lines in dialogs need the role's `waits_for_send` in the role catalogue read.

## Nav (N12, N18; 3a picked)
- ADVANCED_NAV: rename leaf "Access map" to "Insights" (/insights, /graph redirects). If 3a: move "Automatic rules" and "Insights" out of the group to top level after Bundles; rename group "Automation" to "Changes"; rename leaf "Settings" to "Roles that wait" (/automation/settings keeps working, redirects to /changes/roles-that-wait or similar).
- AccountSheet: add a row after Appearance (and Install as an app), same rounded-inner bordered shape, "Sound when drift is found" with "In this browser only" (Segmented Off · On, sm) with the state line from Automation settings 3; remove the sound card from the page.

## Waits-for-you mark (N18, 2c)
- New component `WaitsMark`: 9px circle, 1.5px --muted border, left half --muted; role="img", aria-label "Waits for you", tabIndex 0, tooltip on hover and focus: "Waits for you: when someone gains or loses this role, the change waits for you to send it." Prop `withLabel` renders the words beside it.
- Mark only, after the role name: role picker rows, Requests (Asked for), a person's role rows, a bundle's role rows.
- Mark with words: a project's When given column, rule editor role fields, the picker legend.
- The settings page becomes "Roles that wait for you" (eyebrow Changes); every "Set in Automation settings →" becomes "Set in Roles that wait →".

## Member pages (Area 11)
- Storage, held account (#79): headline "Your account is switched off for now"; body "Makerspace staff have put your TrueNAS account on hold, so it turns sign-ins away. Your files are kept and your access is unchanged. It switches back on when the hold is lifted; there's nothing to do on your side."; aside "On hold since {date}"; status "Online · your account is on hold" (neutral dot); hide addresses and the guide; password row: "Your password can't be changed while the account is on hold. The one you have works again when the hold is lifted."
- Storage, no host (#80): headline stays "Your account is ready"; body "Sign in with your account name and storage password. The address to connect to isn't set up in Syndra yet, so ask makerspace staff for it."; no addresses, no guide.
- My access 5 (N19): a member's approval whose outbox row is unsent shows in Recent changes as Approved (warn dot) with "It isn't on your account yet: makerspace staff check this role before it reaches you, and nothing more is needed from you." When wait_reason is halted:*, the second clause becomes "Syndra couldn't reach the sign-in system when it was approved; staff will send it." The role's row in its room: "Not there yet · approved {when}, and {reason}". Needs the member-scoped outbox read (no operator detail).

## Mobile (Area 12, route-mobile)
1. **Freshness (#86, 86c picked):** in `ReadFreshness`, the provisional dot becomes `border border-warn` (was `border-muted`). Nothing else changes.
2. **Filters below tablet (#87, 87c picked):** `FilterPanel` renders its children in a `Modal size="sm"` with ModalHeader "Filters" below tablet; footer: ghost "Clear all", accent "Show {n} {noun}" (the list's live count after these filters, closes the sheet). Needs `resultCount` and `noun` props. Popover unchanged from tablet up. Search stays inline; no overlay.
3. **Insights below tablet (N20, 20a picked):** hide the Map/List switch and render List; the lens FilterPills become a Select ("Lens: People"…). At rest, list the findings (drift, waiting for you to send, single route, unused) as rows with their link. With a question, render the panel full width: eyebrow, title, sentences, link, Pin, Copy as sentences. State stays in the URL.
4. **Copy check (#84):** the bundle's busy grabber reads "Applying your change — this sheet can't be closed until it finishes." and the grabber's label "Close this sheet"; the consensus records "Working — this can't be closed yet." and "Dismiss". Keep whichever is in main; make them match the ledger.
5. **Nouns (#89):** any stacked row that says "holders" says "people".

## Login (Area 13, route-login)
1. **Sign in again in a new tab (N21, 21b picked):** when an outcome is `SESSION_ENDED`, render under the ActionOutcome a link "Sign in again in a new tab →" (`target="_blank"`, `/auth/zitadel?then=close`) and the faint line "This tab stays as it is. Come back here when you're signed in." `/auth/callback` carries `then=close` through the PKCE cookie like `next`; when present, `/login` renders the door in its opened scene with the handoff slot reading "You're signed in again." / "Go back to the tab you were working in and press the same button. You can close this one." and no action.
2. Nothing else: #93–95 are as built.
3. **Copy fix, either option (outcome.ts):** the `sessionEnded` detail must not repeat `NOTHING_CHANGED`, which ActionOutcome already appends to refused and failed outcomes. New detail: "What you were doing is still here. Sign in again, then press the same button once more." Check the `offline` detail the same way ("Nothing was sent." stays: it says something different).

## States (Area 14, route-states)
1. **The way out of a dialog with a result (#96, 96b picked):** every `succeeded(outcome) ? "Done" : "Cancel"` and `gone ? "Done" : …` becomes "Close" (bundles, policies, expiring access, zitadel projects, CreateRoleDialog, HoldDialog, UnexplainedAccess, RemovalDialog, AddRolesToBundle, GrantDirectAccess). Tests that query `{ name: "Done" }` follow. FilterPanel's "Done" is unchanged.
2. **Banners become Notices (#97, #98):** AppShell's sticky band (OfflineBanner, DegradedBanner) renders inside `<main>` above the page, using the Notice anatomy (proposed/Notice.md). Offline: tone neutral. Sample data: tone warn, title "{n} things on these screens are sample data.", body "People and projects are real. Some bundles, rules, access and history are sample data, and nothing else on screen tells them apart, so treat anything you can't confirm as sample until it's cleared.", then a disclosure "For whoever runs the Syndra server" holding the seed_active line and CommandBlock `tone="warn"`. Remove the "!" glyph and the solid amber fill.
3. **error.tsx and not-found.tsx (#101):** error.tsx renders the danger Notice ("This page couldn't be shown.", the built sentence ending "Nothing was changed.", Reference CopyableValue, "Go to the home page →" as a violet text link, still a plain `<a href="/">`). not-found.tsx renders EmptyState with the built copy and the same link. The filled pill links go.
4. **ActionOutcome, queued (standing rule, either option):** the queued label "Waiting to be sent" becomes "Waiting for you to send", and QUEUED_GLOSS goes; in its place the one violet link "Waiting for you to send →" to Pending changes (proposed/NextStep.md), so the message says what happened ("Given in Syndra") and the detail says why. Remove the space before the label's period. EmptyState already appends "→" to `action.label`; labels passed to it carry none.
5. **Zitadel down (N23):** DegradedBanner renders the last-known Notice when `last_read_at` is set and the never-read Notice when it isn't; delete the "These numbers are not real." copy. ListStates treats a 503 DIRECTORY_UNREACHABLE as never-read: no card, no skeleton, no EmptyState. PageHeader meta that counts people or roles is omitted until a read exists. Backend: B11.

## Additions: benched
Role sheet, Jump to anything, Send one batch and Daylight (N24–N27) are benched until the owner returns to them. Do not build them in this round.
