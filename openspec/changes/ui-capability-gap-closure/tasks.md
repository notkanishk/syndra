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

## 3. B — built, unreachable ✅

- [x] 3.1 **B1** Shadow password vault UI — `<ShadowCredential>` on Member · My access (not the
      System page the brief named; the endpoints are self-only — see design Decision 9). States
      that it is not the institutional login, that nothing reads it yet, and that it cannot be
      read back. Complexity is judged only by the backend, whose sentence is shown verbatim.
      Proxy allowlist extended to the vault routes (self-only) — without it the card was
      unreachable and, because it suppressed its own read error, invisible. See design Decision 11
- [x] 3.2 **B2** **Deleted.** Endpoint, handler, `db.GetRecentCascades` and its three tests.
      `models.CascadeSummary` stays as the per-write shape inside a `CascadeGroup` — see design
      Decision 10

## 4. C — missing lifecycle (backend + UI)

- [x] 4.1 **C1** `DELETE /rules/mapping/{id}` — `CascadeRuleDeleted` is `CascadeRuleUpdated` with
      no replacement edge, committed with `DeleteMappingRuleAndEnqueue`. The confirmation takes
      over the editor rather than stacking a dialog on it
- [x] 4.2 **C2** `PUT`/`DELETE /bundles/{id}` — rename runs no cascade and publishes no version;
      delete loops over holders because coverage is a property of a person. Migration `000021`
      drops the `onboarding_triggers` foreign key that would have blocked it (design Decision 7)
- [x] 4.3 **C3** `POST /requests/{id}/withdraw`, self-only in the handler AND in the statement.
      Migration `000022`. Closed two latent bugs on the way: the decision guard enumerated
      statuses instead of testing `!= pending`, and both views read "settled, not approved" as a
      denial (design Decision 8). Proxy allowlist extended to the withdraw route, and its
      blanket `requester_id` body injection scoped to `POST /requests` — see design Decision 11
- [x] 4.4 **C6** `audit_logs.cascade_id` (migration `000023`), stamped inside
      `enqueueCascadeRows` — which already minted the id and discarded it. Eleven audit inserts
      moved inward, so the invariant is structural rather than remembered, and a source-coherence
      guard fails if a twelfth reappears. `traceFor` gives the column three honest shapes; old
      rows keep their object id, unlinked, and are NOT backfilled by timestamp (design Decisions
      12 and 13). Change history answers `?cascade=` in the query, and says so when the writes
      have been cleared.
      **Caught in review:** the stamp was gated on "did this cause writes" rather than "will that
      screen show them", so a direct grant's removal linked to a page whose query excludes
      `source='direct'` — and the empty state confidently said its still-pending revoke had been
      carried out. Stamp and filter now read one `cascadeGroupSources` list, asserted across the
      `services`/`db` boundary where the bug lived (design Decision 16)
- [x] 4.4b Every audit action has a sentence. `bundle.updated`, `bundle.deleted`,
      `mapping_rule.deleted`, `access_request.withdrawn` — plus `bundle.version_published` and
      `bundle.holder_moved`, which had been rendering as machine keys since bundle versioning
      shipped. `describeAction`'s raw-key fallback is right and silent, so the map is now checked
      against the Go sources in both directions (design Decision 17)
- [x] 4.5 **C7** Settled: an application lives in exactly one project, matching Zitadel and the
      UNIQUE constraint the schema already carries. The design diagram was the only thing claiming
      otherwise. Reopens on a real integration needing roles from two projects in one token —
      design Decision 14
- [x] 4.6 **C9a** Advanced shows **Zitadel's** grant id per project (MkAuth's own row was already
      there, and answers a different question). Operator-only and not fetched otherwise; a project
      with no upstream grant says so rather than showing a dash — design Decision 15
- [ ] 4.7 **C9b** Hardware sync state on the person page. **Not buildable:** there is no per-user
      sync state while the bridge is parked, and a panel that invented one would be the failure
      `/system/hardware-sync` exists to avoid. Blocked on the same contract as 5.1
- [x] 4.8 **C4** Expiry acknowledgement, with the **reopens-when-the-grant-changes** rule.
      Migration `000024`. The rule is a stored `acknowledged_expires_at` and one join condition
      comparing it to the grant's current date — no trigger, no sweep, nothing to forget, and
      verifiable without a database (design Decision 18). The write is checked under `FOR UPDATE`
      and a stale page gets `409` rather than a stored acknowledgement that would never apply.
      Per-row only and grouped rather than hidden; Extend stays on an acknowledged row and its
      checkbox does not (design Decision 19). The old "why there's no second button" copy is
      deleted, not softened — it had become false
- [x] 4.9 Bulk extend acts on the grants that were ticked. `grant_ids` on the bulk contract, and
      refused on every other op. The queue's rows are grants and the contract was keyed on people,
      so ticking one row renewed every expiring grant that person held — including dates months
      outside the screen's window (design Decision 20)
- [x] 4.10 Every write that moves an expiry drops the queue built from expiries. `useCreateGrant`
      and `useApplyBulk` both now invalidate `review`; the second was unreported and had every key
      root except the screen a bulk extend is launched from (design Decision 21)
- [ ] — **C5**, **C8** deferred with their trigger conditions recorded; see the audit doc

## 5. E — live deployment

- [ ] 5.1 **E1** `/system/hardware-sync` distinguishes a deliberately parked sync from a broken one, instead of stating "not connected yet" as a static fact

## 6. Doc drift found by the audit

- [x] 6.1 `feature-coverage.md` — "rule edits flow through DELETE+CREATE". `PUT` is the edit
      path; `DELETE` now exists and is a retirement, not half of an edit
- [ ] 6.2 `feature-coverage.md` — theme toggle / mode persistence "not evidenced"; both exist
- [ ] 6.3 `feature-coverage.md` — audit actor "demo/static in UI flows"; resolved via the name resolver
- [ ] 6.4 `ROADMAP.md` Phase 5 — Welcome Bundle Configuration listed open; it shipped
- [x] 6.5 `advanced-role-crud` — `DisplayLabel` references updated to record the relocation
