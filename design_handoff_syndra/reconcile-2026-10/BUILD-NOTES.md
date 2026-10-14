# Build notes: the October reconcile, read against the code

Written 14 Oct 2026, after reading the repo at `8c46586`. **Where this file and the package disagree about what the system does, this file stands** (ledger row 16). It covers the TrueNAS slice first: Storage (B1, B2), Connected systems (B3, #57). Notes for the other screens are added as each step is built.

## What arrived, verified

| Check | Result |
|---|---|
| Archive layout | 60 files, all under `design_handoff_syndra/reconcile-2026-10/`, no `..` or absolute paths |
| Boards | Load the DS from `design/_ds/` and React from unpkg; `route-storage` renders |
| Design project | Readable live through DesignSync. `CLAUDE.md` there is the same as `RULES.md`. `handoff/2026-10-reconcile/` holds `benched-additions.md`, which isn't in the zip (benched N24–N27) |
| Repo guard | `design_handoff*` is outside the guard's scope. Fixtures use `makerspace.edu` and the operator's name |

---

## Storage (B1, B2)

**B1: the offline half already exists, under another code.** `handleSetMyCredential` already refuses an unreachable system before dispatching: `503 TARGET_UNREACHABLE` (`backend/internal/handlers/my_storage.go:288`).
- **Kept as is.** The screen branches on the code (row 7). Renaming it to `TARGET_OFFLINE` changes no sentence a person reads.
- **Missing: the paused half.** Today a draining or read-only system gets the password dispatched, and the add-on refuses it (`addons/truenas/lifecycle.go:139`). The member then sees a 202 "did not confirm", or `502 CREDENTIAL_NOT_SET`.
- **Built:** `409 TARGET_PAUSED` in the same `switch`, decided from the `lifecycle` value that `describeMyTarget` already reads.

**B2: there is no confirmed-failure threshold to copy.**
- The operator's health read is live, with no remembered opinion (`handlers/targets.go:107`).
- The operator page shows "Not answering" on a single failed read (`TargetOverview.tsx:600`).
- The only consecutive-failure logic is the write path's circuit breaker: 5 failures, 30s, in memory (`addons/deps.go:33`).
- Nothing records when a system stopped answering, so `reachable_since` has no source today.

**Built, as one rule for both pages:**
- A health monitor probes each active target every 30s and keeps, per target, `checked_at`, the time of the first failure in the current run, and the failure count.
- **Offline is confirmed after 3 consecutive failed probes.** `reachable_since` is the first of them. One slow answer stays green, as the board asks.
- The member page and the operator page read the same monitor, so "Offline since 09:40" means the same on both.
- `ponytail:` it is in memory. After a restart Syndra can't tell until the next probes, and the page shows nothing rather than guess (BUILD-LIST Storage 2). It moves to a table only if restarts during an outage turn out to matter.
- `useMyStorage` has no `refetchInterval` today (`ui/src/lib/queries/useMyStorage.ts:88`), so "this page updates on its own" was not true. It now refetches every 30s while a Notice shows.

**Held account (#79).** `SuppressedEntitlement` has no timestamp (`services/entitlement.go:49`). "On hold since {date}" takes the allowance's own `created_at`, which the hold record already carries.

---

## Connected systems (B3)

**The project belongs to the target, and the database keeps it consistent.**
- `targets` gains `project_id` (migration 000051), unique, as the owner ruled.
- Mapping rows **keep** `project_id`. Fifteen resolution sites join on `(project_id, role_key)` (`db/mappings.go:190`, `services/entitlement.go:120`, `role_members.go:264` …), and none of them needs to change.
- A composite foreign key `target_role_mappings (target, project_id) → targets (target, project_id)` makes a mapping from another project impossible.
- **`MAPPINGS_EXIST` falls out of `ON UPDATE RESTRICT`:** changing a project that mappings point at is refused by the constraint, and the handler turns that into the 409 with the count.
- The uniqueness `(target, project_id, role_key, field)` is already `(target, role_key, field)` once the project is fixed per target. It stays.

**The `zitadel` target is exempt.** It is the source of projects, not a system that serves one (`000026` seeds it). It never asks for a project and never shows the gate.

**The gate covers every write that reaches a person's access on the target, or its mapping set:**
- mappings: create, edit, delete, versions, rollback, and their rehearsals
- `adopt`, `release`, `reconcile`, dormant `sweep`, `revoke-access`
- `entitlements` rehearse and apply, `propagations/drain`
- holds (`POST /allowances` naming the target)
- merge-finding, binding-conflict and log-anchor resolution

**Exempt:** lifecycle (owner, 8 Oct) and every read. The package's list named five of these. It is enforced in the write transactions (`db.ErrProjectRequired`, the same pattern as `ErrTargetNotActive`, `db/entitlement_enqueue.go:26`), so no path can miss it.

**Routes as they are.** The mapping list is `GET /targets/mappings?target=` (`handlers/mappings.go:45`), not `/targets/{t}/mappings`. It keeps its route and returns `409 PROJECT_REQUIRED` while the target has no project.

**Values from the add-on.**
- The manifest gains `mappable_fields: [{name, label, enumerate}]` on `EntitlementField`'s schema (`backend/internal/addons/manifest.go:69`).
- An add-on that doesn't declare it falls back to today's rule: every non-lifecycle field, labelled by its name.
- `GET /targets/{t}/mappable-values?field=` serves the last good list, from memory, with `read_at` and `current`.
  - `ponytail:` in memory. A restart during an outage shows "TrueNAS hasn't listed its groups yet" until it answers.

**Built-in groups never reach the picker.** `group.query` returns TrueNAS's built-in groups (`builtin_administrators`, `wheel` …) alongside the makerspace's (`addons/truenas/subjects.go:213`). Mapping a role to one would hand NAS administration to everyone holding it.
- Built-in groups are now outside what Syndra manages: never offered, never added, never removed.
- The values list and the write-path check exclude them, and a subject's reported groups omit them.
- Convergence carries an account's current built-in memberships through its full replace. Today it writes exactly the mapped set, which strips `builtin_users` (SMB's BUILTIN\Users) from any account in it. That is fixed too.

**A truncated read is now an error.** `groupIndex` asks for `subjectReadCap + 1` rows and never checks the count, so a NAS with more than 5,000 groups gets a silently partial index. Both the values list and convergence now refuse a short read, rather than act on half a list.

**Codes as they are:**
- A group that went away still refuses as `400 VALIDATION_FAILED` with `details.value = "unresolvable"` (`handlers/mappings.go:357`), not `422 VALUE_NOT_FOUND`. The `near` suggestions go, as the package says.
- Fail-open is reported as `value_checked: false` (`handlers/mapping_plan.go:116`), not `checked`.

**Meta line.** The system page's meta line shows only the product version (`TargetOverview.tsx:83`), so there is no project there to remove. The census line keeps its sentence and gives up its link to the setup strip (`TargetOverview.tsx:1272`).

## Assign an account (#57)

- `useUsers(q)` (`ui/src/lib/queries/useUsers.ts:149`) already searches by name or email, and returns title, team and avatar. There is no shared single-choice row; the dialog builds one from `SelectionSeat`.
- "Already has {username} on {target}" needs the target's bindings, which `useUsers` doesn't carry. The dialog reads the target's existing inventory read and marks matches.
- `ConfirmByTyping` stays (`components/ui/Acknowledge.tsx:99`).

---

## Questions sent to design

- `to-design/Q-01-truenas-states.md`

## Owner's answers (14 Oct 2026)

1. **Existing mappings are inferred.** A system whose mappings all sit in one project gets that project. A system whose mappings span several stops migration 000051 with a list of them, to be fixed by hand. Nothing is guessed or deleted.
2. **The picker offers every group made on the NAS**, minus the built-in ones. It assumes no naming convention.
3. **Composed roles come later.** Today TrueNAS has only the wide base groups. An administrator-facing way to compose roles is owed, with details still to settle (`openspec/NEXT.md`, "TrueNAS add-on — owed after the October reconcile"). It is blocked on the untraced `EnforceMappingRules` path, because the first composed role is the first mapping rule in production.
4. **ACL mode is unknown.** It is being read from the dev NAS, read-only. The "what this group can open" line waits on that.
