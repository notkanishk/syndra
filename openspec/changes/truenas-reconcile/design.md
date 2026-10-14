## Context

Roadmap **Phase 4, the Target Plane**. This builds the TrueNAS slice of the October reconcile.

**Sources:**
- `design_handoff_syndra/reconcile-2026-10/BUILD-LIST.md`: Storage, Connected systems, member pages
- `proposed/Mapping-implementation-notes.md`
- `BUILD-NOTES.md`: overrides the package where the package misread the code

**Related:** `changes/addon-platform` (manifest, transport, circuit breaker) and `changes/role-mapping-query-shape` (the scanner fix this builds on). The bridge protocol is `design_handoff_syndra/BRIDGE.md`.

**Where things stand:**
- **Health:** a live read per page view (`handlers/targets.go:107`, `my_storage.go:230`). Nothing remembers when a system stopped answering.
- **Mappings:** each carries its own `project_id` (`000029`). The dialog takes a free-text field and value.
- **Group enumeration:** `addons/truenas/subjects.go:211` returns built-in groups, and never checks whether it hit `subjectReadCap`.

## Goals / Non-Goals

**Goals:**
- Every claim the storage and system pages make is one Syndra can back:
  - offline is confirmed, not guessed
  - a password is refused when nothing would receive it
  - a group offered is a group the NAS has and an operator may hand out
- One Zitadel project per target, so a mapping is a role and a group, held consistent by the database.
- The add-on stays the only authority on which fields are mappable and which values exist.

**Non-Goals:**
- Composed roles and role kind (owner: later; `openspec/NEXT.md`).
- The other reconcile steps (foundations beyond `Notice`, bundles, Insights, confirmation). Each gets its own change.
- The "what this group can open" line. It waits on the NAS's ACL mode (owner unsure; reading it needs the owner's approval).
- Renaming the circuit breaker's "Paused after failures". That waits on Q-01.

## Decisions

**D1 · Health monitor, in memory.**
- A `periodic.Runner` probes each active add-on target every 30s with the existing health read.
- Per target it keeps `checked_at`, `consecutive_failures`, and `first_failed_at`.
- **Offline is 3 consecutive failures**, and `reachable_since` is `first_failed_at`.
- Before the first probe after a restart, the state is `unknown`, and pages show no status.
- *Alternative:* a table. Rejected for now, because the only loss is ~90s of "can't tell" after a restart, which the board already draws. `ponytail:` comment names the upgrade.
- It is independent of the write-path breaker, which counts dispatch failures and must keep doing so.

**D2 · `TARGET_PAUSED` before dispatch.**
- `handleSetMyCredential` refuses `lifecycle ∈ {draining, read_only}` with 409.
- `TARGET_UNREACHABLE` keeps its name: the screen branches on the code, and no sentence changes.

**D3 · The project is a target column, and mappings keep theirs.**
- `targets.project_id TEXT UNIQUE NULL`.
- `UNIQUE (target, project_id)` on `targets`, plus `FOREIGN KEY (target, project_id) REFERENCES targets (target, project_id) ON UPDATE RESTRICT` from `target_role_mappings`.
- Resolution code joins on `(project_id, role_key)` at fifteen sites, and none of them changes.
- `MAPPINGS_EXIST` is the constraint's refusal, translated with a count. A cross-project mapping can't be written by any path.
- *Alternative:* drop `mapping.project_id`. Rejected: fifteen rewritten joins, and version-entry history keyed on it.

**D4 · Migration 000051 infers, and refuses to guess.**
- For each target whose mappings name exactly one project, set it.
- If any target names several, `RAISE EXCEPTION` listing target and projects, so the operator fixes them first.
- A target with no mappings stays NULL.

**D5 · The `PROJECT_REQUIRED` gate is raised in the write transactions.**
- `db.ErrProjectRequired` is checked where `ErrTargetNotActive` is (`db/entitlement_enqueue.go:26`), plus the handler entry of path-keyed target writes, through one helper `requireTargetProject(ctx, target)`.
- Lifecycle and every read are exempt, and so is the `zitadel` target.
- The full write list is in `BUILD-NOTES.md`. Each write gets a test that it refuses.

**D6 · `mappable_fields` in the manifest, with a fallback.**
- `EntitlementSchema` gains `mappable_fields []{name, label, enumerate}`.
- An add-on without it falls back to every non-lifecycle field, labelled by name.
- TrueNAS declares `[{name: "group", label: "Group", enumerate: "values"}]`.

**D7 · Last-known values, in memory.**
- `GET /targets/{t}/mappable-values?field=` calls the add-on's values endpoint.
- On success it caches `{values, read_at}` per (target, field) and returns `current: true`.
- On failure it returns the cached list with `current: false`, or `409 VALUES_NEVER_READ` when there is none.
- The write-path check is unchanged and still fails open (`value_checked: false`).

**D8 · Built-in groups are outside the managed set.** Syndra never offers, adds or removes a `builtin: true` group.
- `handleValues` and the value check skip them, so a mapping to `builtin_administrators` is refused as unresolvable.
- A subject's reported groups omit them, so no diff ever sees them.
- Convergence writes the mapped groups **plus the account's current built-in memberships**, so a full replace carries them through. Today's converge writes exactly the mapped set and would strip `builtin_users` (SMB's BUILTIN\Users) from any account in it: a latent defect this closes.
- `groupIndex` refuses a result of `subjectReadCap + 1` rows.

**D9 · UI to the boards.**
- `Notice` lands in `components/ui` as the one dysfunction component (`proposed/Notice.md`), used here first.
- `MyStorage` is rebuilt per BUILD-LIST "/storage" 1–10 and the phone notes.
- `AddMappingDialog` loses its Inputs and gains the role picker scoped to the target's project, plus a value Select from `mappable-values`.
- The project gate, setup strip and Connected systems project column follow `route-system-targets-id`, `route-system-targets-id-mappings` and `route-systems`.
- `AdoptPanel` becomes the #57 Modal.

## Risks / Trade-offs

- **Migration 000051 can stop a deploy** when a target spans projects. This is intended: the operator ruled "infer, don't guess". The exception text names what to fix.
- **In-memory state resets on restart.** That is acceptable (D1, D7), and the pages say "can't tell" rather than guess.
- **Manifest contract change.** Both binaries live in this repo. The backend accepts either shape, so the order they deploy in doesn't matter.
- **Q-01 is open.** The operator-side intermediate state uses the member's rule (green until confirmed) until design answers.

## Verification

```bash
cd backend && go test ./... && go vet ./...
cd addons/truenas && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
```

The live-DB migration test runs through the `SYNDRA_TEST_DATABASE_URL` harness (the dev box Postgres tunnel).
