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
- [ ] 1.6 (with 4.1) Manifest declares `mappable_fields: [{name: group, label: Group, enumerate: values}]`. Test the manifest shape

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

- [ ] 2b.1 Monitor keeps the last three results, `missed_since` (the first miss of the current run) and `started_at`. Tests
- [ ] 2b.2 One backend `status` with A-01's precedence: `cannot_connect` > `offline` > `waiting_to_retry` > `checking` > `answering`. It goes on the targets list and the health read. Test for each pairing
- [ ] 2b.3 Values answer carries `builtin_count` and two built-in example names, for "TrueNAS has no groups of its own yet". Test
- [ ] 2b.4 UI renames at the A-01 call sites, with their tests. Check pips (aria-label) on Connected systems and the system page

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

- [ ] 4.1 `EntitlementSchema.MappableFields`, with a fallback to non-lifecycle fields when it's absent. Tests for both manifests
- [ ] 4.2 `GET /targets/{t}/mappable-values?field=`, with the last-known cache, `read_at` and `current`, and `409 VALUES_NEVER_READ`. Tests: fresh, stale and never read
- [ ] 4.3 Drop `near` from the unresolvable refusal. Update the handler test

## 5. UI: Notice and member storage (D9, BUILD-LIST "/storage")

- [ ] 5.1 `components/ui/Notice.tsx` per `proposed/Notice.md` (amber, red, neutral; `role="alert"` when it blocks). Unit test
- [ ] 5.2 Rebuild `MyStorage` to `route-storage`:
  - the state line, and one display line per state
  - one block per share with `CopyableValue` addresses
  - the neutral usage bar, "nearly full" from 90%, "no limit set", and offline unavailable
  - the password as a next step, and the quiet "Change password"
  - one Notice per card
  - the paused and offline refusals, the held state (#79) and the no-host state (#80)
- [ ] 5.3 `useMyStorage` refetches every 30s while a Notice shows
- [ ] 5.4 `ConnectionGuide` Modal, with device from the user agent, hidden while offline
- [ ] 5.5 Phone notes: three-segment step bar, full-width guide button, password field ≥ 220px
- [ ] 5.6 Tests: each state's copy and controls, never 0 usage offline, no password field when paused or offline

## 6. UI: Connected systems (D9, B3, #57)

- [ ] 6.1 Project gate (3a, frame `noproject`):
  - violet Step 1 of 2 panel with the picker
  - taken projects disabled and named
  - role counts
  - waiting regions inert at 40%
  - result frame `projectset`
- [ ] 6.2 Setup strip: Zitadel project link, mapping summary, and "Mappings →" / "Add the first mapping →". The census line drops its link and names the project
- [ ] 6.3 `AddMappingDialog`:
  - role picker scoped to the project, unmapped roles only
  - field label from the manifest, with a Select only for two or more fields
  - value Select from `mappable-values`, with the stale label, "TrueNAS hasn't listed its groups yet", and the re-read on unresolvable
- [ ] 6.4 Mapping rows drop the project prefix
- [ ] 6.5 Connected systems shows the project read-only. A null project shows "Choose its Zitadel project →" under Needs you
- [ ] 6.6 #57 Assign Modal:
  - people search with single-choice rows
  - "Already has {username} on {target}"
  - the Changes / Stays box
  - `ConfirmByTyping`
  - "Assign {account} to {name}"
- [ ] 6.7 Tests for the gate, the strip, the dialog states and the assign Modal

## 7a. Mapping dialog (owner, 14 Oct)

- [x] 7a.1 Plan rows resolve names by id, so "Name not available" is gone from every plan screen
- [x] 7a.2 Mapping rehearsal copy in plain language
- [x] 7a.3 Every 500 answers with a sentence and a support reference; the cause is logged under that reference
- [x] 7a.4 RoleMapping reads share one column list and one scan list, checked to match

## 7. Close the loop

- [ ] 7.1 Full verification: backend, add-on and UI suites, plus lint and build
- [ ] 7.2 `design_handoff_syndra/reconcile-2026-10/SHIPPED.md` for this slice
- [ ] 7.3 Owner reruns `/design-sync` so `Notice` and `ConnectionGuide` reach the design system
- [ ] 7.4 Fold in A-01 from design (operator intermediate state, "paused" wording, NAS with no groups of its own)
- [ ] 7.5 Codebase-memory re-index (`detect_changes`)

## 8. Where to connect, from the NAS (WALKTHROUGH Part 3)

- [ ] 8.1 Add-on: `reach` read. It covers SMB shares, plus ACLs at each share root and first-level folder. For a set of groups it returns `{share, path, access: read|modify, dataset, addresses}`, cached per share and refreshed on reconcile. Tests: traverse-only isn't reach; Community resolves to `main/waddle_share` on the production-shaped fixture
- [ ] 8.2 Manifest: declare the operation. Backend: `GET /me/targets` items carry `reach` from the member's own groups, never from the group names. Tests
- [ ] 8.3 Operator: `GET /targets/{t}/groups/{group}/reach` for the mappings page's "what this group opens" line. Test
- [ ] 8.4 `storage.status` reports usage per reachable path from its dataset, with 0 rather than absent. Test
- [ ] 8.5 UI: share blocks render from `reach`, and the `smb://host/group` derivation is deleted

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
- [ ] P2 Two Syndra-created accounts are still `smb: false`, with no password yet. Once the fix deploys, their first password switches SMB on. The test account was fixed by hand.
- [ ] P2 Share permissions must come from the role groups, never `builtin_users`. Syndra deliberately never adds members to it (D8). The NAS-side permission changes are with the owner.
- [ ] P2 NAS hardening and data-protection findings that aren't Syndra's own were reported to the owner directly. They are deliberately not recorded in this public repository.
