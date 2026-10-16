## 1. TrueNAS add-on: built-in groups and truncation (D8)

- [x] 1.1 `groupIndex` selects `builtin`, refuses `subjectReadCap + 1` rows, and returns which group ids are built in
- [x] 1.2 `handleValues` and the value-resolve path exclude built-in groups
- [x] 1.3 A subject's reported `Groups` omit built-in groups
- [x] 1.4 `converge` writes the mapped groups plus the account's current built-in memberships. `sameSet` compares only managed groups
- [x] 1.5 Tests:
  - an admin group is unresolvable
  - `builtin_users` survives a converge and isn't reported
  - an over-cap read errors in enumeration and in convergence
- Found on the way: `plan` never resolves group names (already true for an unknown group). A legacy mapping to a built-in group plans as a change, then `apply` refuses it with 422 naming the group. New mappings can't name one: the backend checks values against the filtered list. That leaves it loud, not silent. Making plan resolve names costs a group read per plan.
- [x] 1.6 Superseded: the schema itself says what is mappable. Every non-lifecycle field is, with `label`, `single` and `per_person`. Was: manifest declares `mappable_fields: [{name: group, label: Group, enumerate: values}]`. Test the manifest shape

## 2. Backend: health monitor (D1) and the paused refusal (D2)

- [x] 2.1 `addons.HealthMonitor`:
  - per-target `checked_at`, `consecutive_failures`, `first_failed_at`
  - `State(target)` returns `online | offline(since) | unknown`
  - a `periodic.Runner` probes every active add-on target every 30s
- [x] 2.2 Tests:
  - one failure stays online
  - three failures are offline since the first
  - recovery clears the run
  - no probe yet is unknown
- [x] 2.3 `GET /me/targets` items gain `checked_at` and `reachable_since`. `reachable` comes from the monitor (as `health`: `online | offline | unknown`, with `reachable` true only when online). Handler tests
- [x] 2.4 `handleSetMyCredential` refuses `draining` / `read_only` with `409 TARGET_PAUSED` before dispatch. Test that nothing is dispatched
- [x] 2.5 The operator health read and `GET /api/v1/targets` expose the monitor's state beside the live read. Tests

## 2b. A-01 from design: the states between answering and offline

- [x] 2b.1 Monitor keeps the last three results, `missed_since` (the first miss of the current run) and `started_at`. Tests
- [x] 2b.2 One backend `status` with A-01's precedence: `cannot_connect` > `offline` > `waiting_to_retry` > `checking` > `answering`. It goes on the targets list and the health read. Test for each pairing
- [x] 2b.3 Values answer carries `builtin_count` and two built-in example names, for "TrueNAS has no groups of its own yet". Test
- [x] 2b.4 UI renames at the A-01 call sites, with their tests. Check pips (aria-label) on Connected systems and the system page

## 3. Backend: one project per target (D3–D5)

- [x] 3.1 Migration 000051:
  - `targets.project_id` (unique) and `UNIQUE (target, project_id)`
  - infer from mappings, and `RAISE EXCEPTION` naming targets that span several projects
  - composite FK from `target_role_mappings` with `ON UPDATE RESTRICT`
  - down migration
- [x] 3.2 Live-DB migration tests: infer one; refuse two, naming both; a mapping to a foreign project is refused by the FK
- [x] 3.3 `PUT /targets/{t}/project`:
  - `409 PROJECT_TAKEN` names the system holding it
  - `409 MAPPINGS_EXIST` carries the count
  - the `zitadel` target is refused
  - audit line
  - Tests for each
- [x] 3.4 `requireTargetProject` and `db.ErrProjectRequired` across the BUILD-NOTES write list. Lifecycle, reads and the `zitadel` target are exempt
- [x] 3.5 A table test asserting every gated route refuses with `409 PROJECT_REQUIRED` and lifecycle doesn't
- [x] 3.6 Mapping create and rehearse-create take the project from the target. `GET /targets/mappings` returns `409 PROJECT_REQUIRED` without one. Tests
- [x] 3.7 `GET /api/v1/targets` returns `project: {id, name} | null`. Test

## 4. Backend: mappable values (D6–D7)

- [x] 4.1 Superseded with 1.6: mappable = not lifecycle, so there is no second declaration to fall back from
- [x] 4.2 `GET /targets/{t}/mappable-values?field=`, with the last-known cache, `read_at` and `current`, and `409 VALUES_NEVER_READ`. Tests: fresh, stale and never read
- [x] 4.3 Drop `near` from the unresolvable refusal, together with 6.3: until the picker ships the dialog is free text, and `near` is what answers a typo. Update the handler test

## 5. UI: Notice and member storage (D9, BUILD-LIST "/storage")

- [x] 5.1 `components/ui/Notice.tsx` per `proposed/Notice.md` (amber, red, neutral; `role="alert"` when it blocks). Unit test
- [x] 5.2 Rebuild `MyStorage` to `route-storage`:
  - the state line, and one display line per state
  - one block per share with `CopyableValue` addresses
  - the neutral usage bar, "nearly full" from 90%, "no limit set", and offline unavailable
  - the password as a next step, and the quiet "Change password"
  - one Notice per card
  - the paused and offline refusals, the held state (#79) and the no-host state (#80)
- [x] 5.3 `useMyStorage` refetches every 30s while a Notice shows
- [x] 5.4 `ConnectionGuide` Modal, with device from the user agent, hidden while offline
- [x] 5.5 Phone notes: three-segment step bar, full-width guide button, password field ≥ 220px
- [x] 5.6 Tests: each state's copy and controls, never 0 usage offline, no password field when paused or offline

## 6. UI: Connected systems (D9, B3, #57)

- [x] 6.1 Project gate (3a, frame `noproject`):
  - violet Step 1 of 2 panel with the picker
  - taken projects disabled and named
  - role counts
  - waiting regions inert at 40%
  - result frame `projectset`
- [x] 6.2 Setup strip: Zitadel project link, mapping summary, and "Mappings →" / "Add the first mapping →". The census line drops its link and names the project
- [x] 6.3 `AddMappingDialog`:
  - role picker scoped to the project, unmapped roles only
  - field label from the manifest, with a Select only for two or more fields
  - value Select from `mappable-values`, with the stale label, "TrueNAS hasn't listed its groups yet", and the re-read on unresolvable
- [x] 6.4 Mapping rows drop the project prefix
- [x] 6.5 Connected systems shows the project read-only. A null project shows "Choose its Zitadel project →" under Needs you
- [x] 6.6 #57 Assign Modal:
  - people search with single-choice rows
  - "Already has {username} on {target}"
  - the Changes / Stays box
  - `ConfirmByTyping`
  - "Assign {account} to {name}"
- [x] 6.7 Tests for the gate, the strip, the dialog states and the assign Modal

## 10. Home folders by role (owner, 14 Oct)

- [x] 10.1 Add-on schema: a single-valued mappable field `home` (label "Home folder"). Its values are folders inside an SMB share, read from the NAS. Tests
- [x] 10.2 Converge:
  - no home yet: `home = <place>/<account name>`, with `home_create: true` and `home_mode: "700"`. That is TrueNAS's own private create, mode 700 with the ACL stripped.
  - already there: no change.
  - elsewhere: never moved. The outcome says so.
  - no longer wanted: kept.
  - Tests for each
- [x] 10.3 Refuse to point a home at a folder that already exists and is owned by another uid. The outcome names the folder. Test
- [x] 10.4 (superseded by 10.8) Two roles naming different places: everything else lands, the home is left as it is, and the outcome says which two. Test
- [x] 10.6 Home folder limit: `home_quota`, a per-role value in bytes, where the largest of a person's roles wins. It is set as a user quota on the home's dataset. A limit nobody gives any more is left as it is, never removed into "unlimited". The add-on key needs TrueNAS's DATASET_WRITE role to set it (owner decision).
- [x] 10.7 Per-person limit: an allowance with direction `set` (migration 000052). It is allowed only on fields the add-on marks per-person, bounded by an expiry or a review date, and applied by the resolver over the roles' value, only for someone the target reaches.
- [x] 10.9 A-03 built: the setup card's Home folders row (choose, change with what stays), the mapping dialog's home folder and limit as one reviewed plan, the row's home line, one person's own limit on the system's account list (set, in force, review due, ended over), Holds due listing a limit under review, the member's "Full" line, "No limit" as an explicit 0 that lifts the limit, and the operator's Account sheet with My access and Network storage. An operator approving their own request is allowed and recorded as `access_request.self_approved` (owner, 15 Oct). Tests
- [x] 10.8 One home location per system (owner, 15 Oct). A role says only whether it gives a home folder (`home: "true"`) and its default limit; the higher limit among a person's roles wins. The location is the add-on's own setting, `home_root`, chosen from the dataset folders at the top of each share and set through Syndra (`GET/PUT /targets/{t}/settings/{name}`, who and when recorded). Changing it moves nothing: homes already made stay. With none chosen, no home is made and the outcome says why. Supersedes 10.4 and A-02 §5/§7. Tests
- [x] 10.5 UI (A-03): the system's home location, the mapping dialog's "Gives a home folder" and limit, one person's own limit, and "Your home folder" first among the member's places

## 11. Identity numbers never reused (found 14 Oct, read-only audit)

TrueNAS gives a new account the next free uid, and gives its personal group the next free gid. It can't see numbers that only remain as owners of old files. Old members' home folders are owned by uids and gids that now belong to new Syndra accounts, so those accounts can open those folders.

- [x] 11.1 The add-on assigns uids from its own range, 200000–999999 by default (`TRUENAS_ID_FLOOR`, `TRUENAS_ID_CEILING`). TrueNAS accepts 0–90,000,000 and can't change a uid later; the range sits clear of the NAS's own accounts, of Proxmox container ids (100000–165535) and of the idmap ranges above 90M. It keeps a high-water mark in its store, so a number is never handed out twice. Test
- [x] 11.2 The personal group gets the same number: `group.create {gid: uid}`, then `user.create {group: <id>}`. If the user create fails, the group create is undone. Test
- [ ] 11.3 Remediation, on the NAS:
  - [x] the 25 orphaned home folders are owned by root and private to the Admin group (14 Oct, with the owner's approval; the state before is kept for undo)
  - [x] the three Syndra-created accounts and their personal groups are deleted on TrueNAS (15 Oct, owner's approval); Syndra makes them again in the new range once the add-on and UI are complete
  - [ ] files under `main` owned by a number no account or group holds are given to root, ACLs untouched, with the owners recorded for undo (owner runs it on the NAS, 15 Oct)
  - [ ] their three bindings are released in Syndra ("Check for differences" on the system page lists them as pointing at nothing; owner, signed in)

## 7a. Mapping dialog (owner, 14 Oct)

- [x] 7a.1 Plan rows resolve names by id, so "Name not available" is gone from every plan screen
- [x] 7a.2 Mapping rehearsal copy in plain language
- [x] 7a.3 Every 500 answers with a sentence and a support reference; the cause is logged under that reference
- [x] 7a.4 RoleMapping reads share one column list and one scan list, checked to match

## 7. Close the loop

- [x] 7.1 Full verification: backend, add-on and UI suites, plus lint and build
- [x] 7.2 `design_handoff_syndra/reconcile-2026-10/SHIPPED.md` for this slice
- [ ] 7.3 Owner reruns `/design-sync` so `Notice` and `ConnectionGuide` reach the design system
- [x] 7.4 Fold in A-01 from design (operator intermediate state, "paused" wording, NAS with no groups of its own)
- [ ] 7.5 Codebase-memory re-index (`detect_changes`)

## 8. Where to connect, from the NAS (WALKTHROUGH Part 3)

- [x] 8.1 Add-on: `GET /reach` reads each enabled share's root and first-level folder ACLs (unsimplified NFS4, in order: the first entry naming a permission decides it). Each place is `{share, folder, path, access: read|change, dataset}`. Per group it is what holding the group adds to what every account opens (everyone@ and builtin_users), which is listed apart. One read serves five minutes; a failed read serves the last one as not current. Tests: ordering, deny-first, inherit-only, group@, an untraversable root, the production-shaped fixture, cache and fallback, never read. Also replayed against the live NAS's own answers (read-only)
- [x] 8.2 `storage.status` carries `places` from the groups the account is actually in, home folder first. `/me/targets` passes them through with both addresses built once from the connection host. No manifest operation: it is part of the member's own status read. Tests
- [x] 8.3 Operator: `GET /targets/{t}/reach`, every group at once, so the card header says the read's age once (A-02 §3). Always 200, with `read: false` for "not read from TrueNAS yet". Tests
- [x] 8.4 Usage per place from its dataset's user quota table, only where the member can change files. No row is 0 ("Nothing saved here yet"); an unreadable table is no usage, never 0. Test
- [x] 8.6 "from your Fabrication lead role" (A-02 §1): the resolver records which held roles gave each value; `/me/targets` names the roles whose groups open each place, from the target's last read
- [x] 8.7 (with A-03) Preview counts homes it will make ("Makes 34 home folders in main › home_dir; 2 people already have one there"). The places offered for the location are the dataset folders at the top of each share, not hidden, labelled `share › folder` [x]
- [x] 8.8 Retired with 10.8: with one location, two roles can't name two home folders
- [x] 8.9 Built as "None of the roles you hold gives a home folder any more" (`home_kept`): which lost role gave it isn't kept, so it isn't named. "You no longer hold Lab Tech, the role that gave you this folder" (A-02 §6): the account's home against what the member's roles want now
- [x] 8.11 `TRUENAS_SHARES`: the SMB shares Syndra considers, empty for every enabled one. Outside it a share is not read for places or home locations, counted for usage or reported for audit coverage. Owner, 15 Oct: `main` only, for now. Test
- [x] 8.10 Access given back: `needs_new_password` from the add-on (account has a password, SMB off, SMB wanted), and `restored_at` on `/me/targets`, the latest end of a deny hold on that target, lifted or lapsed. Tests
- [x] 8.5 UI: share blocks render from `reach`, and the `smb://host/group` derivation is deleted

## 9. Every way of giving a role reaches TrueNAS (WALKTHROUGH G1, G4, G5)

- [x] 9.1 Request approval and every direct grant (add, replace, revoke) converge the one system their project serves (`services.ConvergeProjectChange`, through `grantDirect`). A grant made in Zitadel's own console does **not**: a Zitadel change with no Syndra trace is drift to triage, and the resolver doesn't count it as held.
- [x] 9.1b Safety net: the reconcile pass queues a first convergence for everyone who holds a mapped role and has no account. It skips anyone with an unsent one already waiting, and anyone whose access resolves to disabled.
- [x] 9.2 Creating or lifting a hold queues a convergence for its subject. Test
- [x] 9.3 `user_deactivated` queues a convergence that disables the account. Test

## Found in production, 14 Oct 2026 (read-only audit of drive.makerspace.tools)

The only Syndra writes in 60 days were prod's 3 `user.create` and 1 `user.update`, all on 14 Oct. No group was stripped and no account was locked. Dev made no writes.

- [x] P1 The first password never switched SMB on. `desiredSMB` read the observed snapshot (fixed: `Binding.WantsSMB`). Convergence also never sends a standalone SMB enable now. TrueNAS refuses it and takes the whole update with it.
- [x] P1 Accounts were created with the zsh shell. They now get `/usr/sbin/nologin`, verified against `user.shell_choices`.
- [x] P1 Dev and prod shared the production NAS and its API key, and dev holds an unsent outbox row that would have drained against production. Dev's add-on was stopped on 14 Oct (owner: dev pushes nothing). It restarts on the next `docker compose up -d`. A separate key, or no NAS at all for dev, is still owed.
- [x] P2 The three Syndra-created accounts were switched to nologin on 14 Oct (owner approved). SSH password login, SSH keys and sudo were already off.
- [x] P2 (moot: the accounts were deleted on 15 Oct, 11.3) Two Syndra-created accounts were `smb: false`, with no password yet. Once the fix deploys, their first password switches SMB on. The test account was fixed by hand.
- [ ] P2 Share permissions must come from the role groups, never `builtin_users`. Syndra deliberately never adds members to it (D8). The NAS-side permission changes are with the owner.
- [ ] P2 NAS hardening and data-protection findings that aren't Syndra's own were reported to the owner directly. They are deliberately not recorded in this public repository.

## Found in production, 15 Oct 2026 (Pending changes never drained)

- [x] P1 The plan promised "SMB enabled" for an account still waiting on its first password, a change the apply never makes (the password carries it). Every "Check for differences" queued a new convergence for each such account, each drained to "no change", and Pending changes never emptied. The plan now says what the apply says: no change, waiting for the password. Test
- [x] P2 The periodic pass queued a second row for a bound person who already had one waiting, so one difference was two rows after two passes. One row per person now, the rule the unbound half already kept. Test
- [x] P2 Pending changes showed an add-on convergence as "give /", labelled "Given by hand". The reason each caller writes (`SystemConvergence.Reason`) was never stored. It is now (`propagation_outbox.reason`, migration 53), in plain words: the check's own "Joins Staff.", "Gets an account, for a role they hold.", "Their roles changed." The row reads "TrueNAS account: …", and says who queued it: a person, or Check for differences. Test
- [ ] P2 Releasing an account does not withdraw a convergence already waiting for that person, and the next pass queues a create for anyone who still holds a mapped role. Releasing is not a way to keep someone off TrueNAS; removing their role, or the mapping, is.

## Code review before the 15 Oct deploy

- [x] R1 One row per person skipped a new row while an older one waited, and the drain applies the snapshot a row was queued with, so a stale state (an account enabled for someone deactivated since) could land. It now skips only a waiting row that wants exactly the same thing. Test
- [x] R2 The plan ignored the home folder limit, so a limit-only change was "no change" and never landed, and a failed limit write was never retried. The plan now reads the limit, read-only, the way the apply does. Test
- [x] R3 The plan promised a home the apply refuses (a folder that is someone else's), queued again every pass. It now decides the home with the apply's own rules and says why none is made. Test
- [x] R4 A limit was set on whatever dataset held the home, so a home made by hand at a share's root would cap the person across the whole share. Limits are now set only for a home in a home place. Test
- [x] R5 An account being switched off no longer gets a new home folder, which rode in the same update as the lock and could take it down. Test
- [x] R6 Choosing where homes go is refused against a stale read of the NAS, not only a missing one. Test
- [x] R7 "No limit" lost to any size on the system page's limit line, and read as a 0-byte limit to be over. Test
- [x] R8 Mapping dialog: removals run group last, so one that stops part-way leaves a row the list shows; a part failing after others landed says the role is part-way changed; people waiting are counted once, not once per part. Test
- [x] R9 Role counts on the system page and Mappings counted a role's group, home and limit rows as three roles. Test
- [x] R10 Limit dialog: when the new limit saves and the old one doesn't end, it says so and retries only the ending, instead of saving a third.
- [x] R11 A fractional limit read back rounded to one decimal and was rewritten on an unrelated change. Test
- [ ] R12 Not fixed: `home_root` is not re-checked at apply time or part of the plan's fingerprint, so a place destroyed after it was chosen would get plain directories. Rare; the add-on refuses stale reads when it is chosen.
- [ ] R13 Not fixed: rows a role leaves with no group row (only possible if a removal failed before R8) are not shown with a Remove.

## Found in production, 15 Oct 2026 (one role asked for five times)

test_user asked five times, from one dialog, for Community, which they held for good. Every copy reached the queue, and approving one would have written its 64-day end over their access. Owner, 15 Oct: a request is for a role someone lacks, one per role; nothing merges into what is there.

- [x] D1 One waiting request per person and role: a unique index (migration 54), and a second ask returns the waiting one (409 ALREADY_ASKED). Test (live)
- [x] D2 No request for a role already held, with an end date or without (409 ALREADY_HELD). The dialog marks held and already-asked roles and won't send them. Test
- [x] D3 Approving a request for a role the person has since come to hold is refused, in the single decision and the bulk plan alike; the queue says "Already has this… decline it to close the request" and Approve is off. Test
- [x] D4 The dialog becomes its result once sent, with Done only, so nothing is sent twice from it. The bottom spacing that clipped is fixed with it. Test
- [x] D5 Migration 54 closes, as `system` and audited, waiting requests for roles already held and copies of a waiting request (keeping the earliest). Test (live)
- [x] D6 Giving a role someone already has is refused everywhere: People's grant dialog, the grant endpoint, and bulk assign (no change, including through a bundle). Extend is the one act that changes an existing end (`extend: true`, `db.EndSet`), and only to a later one. Test
- [x] D7 A write that decides no end (bundle and rule cascades, the Zitadel users page, drift Adopt) keeps the end a grant has (`db.EndKept`, the default). They used to write "no end", turning access that ended into access that never does. Test (live)
- [x] D8 Drift Adopt is refused for a finding Zitadel doesn't hold (syndra_only), in the plan and the apply; the button says why. Test
- [x] D9 One hold of a kind in force: a second identical one is refused (409 ALREADY_IN_FORCE), so revoking twice no longer rotates the password twice; a person's own limit replaces the last in one transaction. Test (live)
- [x] D10 Publishing an unchanged mapping set is refused; a duplicate bundle name says so instead of a database error; Manage bundles no longer inverts a change on retry; Extend locks after it lands. Test
- [x] D11 Every dialog opens clean: dialogs with an `open` prop mount only while open (`mountedWhileOpen`); Ask for access reopened on its last result until a reload. A guard fails the old pattern. Test
- [x] D12 A dialog's footer no longer paints over the focus ring of the field above it (`ModalFooter` `mt-2`).
- [x] D13 Grant direct access marks the roles the person already has in its picker, as Ask for access does.
- [x] D14 The revoke sweep waits for the add-ons at start-up, like the reconcile, and the start-up refresh is quiet until then: every deploy logged an unreachable target from the second before the add-on listened.

## Found in production, 15 Oct 2026 (a request overtaken by a grant)

- [x] E1 A grant from any route (direct, approval, bundle, rule cascade) closes a waiting request for the same role in its own transaction, as `fulfilled` (migration 55), reviewer the giver, note naming the route, audited `access_request.closed_as_given`. It stayed open: the queue asked for a decision, the person couldn't ask again, and Decline told them "Not approved". Test (live, mutation-checked)
- [x] E2 Migration 55 turns 000054's "already held" declines into `fulfilled`. Test (live)
- [x] E3 Holding reads the queue: a queued add (a bundle's too, which has no ledger row) counts as held and `waiting`, and the refusal says it may not work yet; a queued revoke means not held, even while Zitadel still lists the role. Test (live)
- [x] E4 An add cancels the queued revoke it answers (any source) and is owed only when the grant isn't standing; giving back before the removal was sent queued two changes that net to nothing. Test (live)
- [x] E5 A person's page read a withdrawn request as "Waiting on a decision"; it uses the queue's words (`requestOutcome`). Test
- [x] E6 A role Zitadel doesn't offer is refused when asked for, given or approved (409 ROLE_NOT_OFFERED), and in the decision plan; an unread or degraded catalogue fails closed (503 CATALOGUE_UNREAD). The API took any project and key; only the pickers kept to the catalogue. Test
- [x] E7 An add identical to one still queued reuses it: extending before the first send no longer queues a second. Test (live)
- [x] E9 Never invisibly (owner, 15 Oct): an operator's action that would cancel a queued, unsent change is refused (409 WOULD_CANCEL_QUEUED, with each change: what, for whom, who queued it, when, why) until they confirm exactly those; `request()` asks one root confirmation and resends with `X-Syndra-Cancel-Ack`. The gate sits at the one place cancellations happen (`withdrawUndelivered`, `db.CancelGate`), so no screen can cancel silently. Plans name the cancellations on their rows and fold the ids into the row fingerprint (a change queued after review makes the plan stale); their apply is marked reviewed. System work and a member's own request aren't asked. Every cancellation is audited `propagation.cancelled_unsent` by the actor. Test (live, handler, UI)
- [x] E10 Removing an unsent direct grant now cancels its queued add: the match required the grant id as reference, which a direct add never carries, so both were queued. Preview and action share one predicate (`answeredWhere`). Test (live)
- [x] E11 The decision plan read withdrawn or fulfilled requests as "will change" and then refused them; a holding it couldn't read let it say approve. Both fixed. Test
- [x] E12 A write that queued nothing (a removal cancelled, the grant still standing) reported "pending"; it reports applied, and nothing tries to drain an empty id.
- [ ] E8 UI sweep owes design's request rows (route-requests): one status seat (dot, word, note), Withdraw outline and last, the role named on a member's row. Today a withdrawn row shows a pill plus "Applied", and a member's row names only the project.
- [ ] E13 Motion built short of the board (Q-05 §4), for the motion sweep after A-05: settle's push-down utility unused, so disclosures open instantly; dialogs, popovers and cards animate in but not out; rows a filter change brings in replay arrive; `ui/src/lib/ui-view.tsx:84` glides with reduced motion on; ActionOutcome, the selection bar and the offline and degraded Notices don't move as Motion 2a, 3a and 6a draw. Since `reconcile-build` step 1: the two system Notices unfold by 6a (`Unfold`), outcomes fade into their slot (2a); the selection bar (3a) and the rest remain.
