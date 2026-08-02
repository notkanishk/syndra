# Tasks — UI Capability Gap Closure

Item ids match [`docs/UI-CAPABILITY-GAPS.md`](../../../docs/UI-CAPABILITY-GAPS.md).

## 1. A9 — role identity composed in one place ✅

- [x] 1.1 `roleLabel(projectName, roleKey, displayName?)` in `lib/format.ts`, replacing the dead `formatRoleRef()`
- [x] 1.2 `<RoleRef>` in `components/names/`, replacing `<RoleName>` (which rendered the role without its project)
- [x] 1.3 Mount `<RoleRef>` at all nine hand-rolled pair sites — Today ×2, RequestsScreen, expiring-access, PersonRequests, ManageBundles ×2, bundles ×4
- [x] 1.4 `GrantDirectAccess` success toast names the pair, not the bare key
- [x] 1.5 `MemberAccess` expiry warning names the pair — it sits outside the project cards that disambiguate the rows
- [x] 1.6 `Makerspace` stops printing the project twice per row
- [x] 1.7 Remove `CatalogRole.display_label` (backend + TS type + tests) — see design Decision 1
- [x] 1.8 Tests: `RoleRef.test.tsx` (pair rendered, same key distinct across projects, em dash over half an identity); `format.test.ts` rewritten for `roleLabel`
- [x] 1.9 `go test ./... && go vet ./...`; `bun run test && bun run lint && bun run build`

## 2. A — remaining regressions ✅

- [x] 2.1 **A1** `describeDrain()` turns a `DrainResult` into one sentence and a tone; both Resume buttons use it. Requeued, errored and each halt reason are named, and only a clean pass reads as success
- [x] 2.2 **A2** `<AddRolesToBundle>` — search across every project, grouped, multi-select, already-held roles shown and disabled, sequential apply that stops at the first failure and leaves the rest selected
- [x] 2.3 **A3** Event activity gains an outcome filter. Raw statuses map to buckets (`outcomeOf`) because the two tables disagree on wording; an unrecognised status never reads as done
- [x] 2.4 **A4** Drift triage gains server-side project and source filters. `user_id` deliberately not offered — see design Decision 2
- [x] 2.5 **A5** Bulk confirmation-mode apply on the rules index, via the shared selection components
- [x] 2.6 **A6** Request duration picker (week / month / end of term), and the ask is now visible to the operator deciding it
- [x] 2.7 **A7** Member catalogue — every project and role, held ones marked, "Ask for this" deep-links the request dialog prefilled
- [x] 2.8 **A8** Keyset pagination on `/audit` — `(created_at, id)` cursor, because `created_at` is the transaction timestamp and a cascade writes several rows at the identical instant
- [x] 2.9 **A10** `<CreateRoleDialog>` extracted; reachable from `/projects/{id}` with the project pinned and stated rather than editable
- [x] 2.10 **A11** Roles index gains a "Used by" column and an Unused filter; Today's "N roles nobody holds" now deep-links to it
- [x] 2.11 **A12** Six superseded zero-consumer hooks deleted, with their orphaned types and query keys

## 3. B — built, unreachable

- [ ] 3.1 **B1** Shadow password vault UI — four endpoints, Argon2id storage, 23 tests, no surface
- [ ] 3.2 **B2** Decide `GET /propagations/cascades`: expose or delete

## 4. C — missing lifecycle (backend + UI)

- [ ] 4.1 **C1** `DELETE /rules/mapping/{id}` — route, repository, UI. A rule authored wrong is currently permanent
- [ ] 4.2 **C2** `PUT`/`DELETE /bundles/{id}` — a bundle cannot be renamed or retired
- [ ] 4.3 **C3** Withdraw an access request. The copy for it is already written in `requests_bulk.go`
- [ ] 4.4 **C6** Carry the cascade id on `audit_logs` so the Trace column stops being an inference
- [ ] 4.5 **C7** Settle app ↔ project cardinality (ISC-45)
- [ ] 4.6 **C9** Person page in Advanced: raw grant ids, hardware sync state
- [ ] — **C4** stays flagged, not built. **C5**, **C8** are deferred by design; see the audit doc

## 5. E — live deployment

- [ ] 5.1 **E1** `/system/hardware-sync` distinguishes a deliberately parked sync from a broken one, instead of stating "not connected yet" as a static fact

## 6. Doc drift found by the audit

- [ ] 6.1 `feature-coverage.md` — "rule edits flow through DELETE+CREATE"; no `DELETE` exists
- [ ] 6.2 `feature-coverage.md` — theme toggle / mode persistence "not evidenced"; both exist
- [ ] 6.3 `feature-coverage.md` — audit actor "demo/static in UI flows"; resolved via the name resolver
- [ ] 6.4 `ROADMAP.md` Phase 5 — Welcome Bundle Configuration listed open; it shipped
- [x] 6.5 `advanced-role-crud` — `DisplayLabel` references updated to record the relocation
