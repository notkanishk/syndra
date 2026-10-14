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
- [x] 10.8 One home location per system (owner, 15 Oct). A role says only whether it gives a home folder (`home: "true"`) and its default limit; the higher limit among a person's roles wins. The location is the add-on's own setting, `home_root`, chosen from the dataset folders at the top of each share and set through Syndra (`GET/PUT /targets/{t}/settings/{name}`, who and when recorded). Changing it moves nothing: homes already made stay. With none chosen, no home is made and the outcome says why. Supersedes 10.4 and A-02 §5/§7. Tests
- [ ] 10.5 UI (after A-03): the system's home location, the mapping dialog's "Gives a home folder" and limit, one person's own limit, and "Your home folder" first among the member's places

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
- [ ] 8.7 (with A-03) Preview counts homes it will make ("Makes 34 home folders in main › home_dir; 2 people already have one there"). The places offered for the location are the dataset folders at the top of each share, not hidden, labelled `share › folder` [x]
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
