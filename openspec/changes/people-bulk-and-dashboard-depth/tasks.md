# Tasks — people bulk operations and dashboard depth

## Track 1 — A name is never an id

- [x] PBD-01 `extractSessionFields` returns `""` rather than falling back to `claims.sub`; whitespace-only claims count as absent.
- [x] PBD-02 `ProfileMetadata` carries `name` and `email`; `fetchProfileMetadata` stops discarding what `/me/profile` already returned.
- [x] PBD-03 `nameFromEmail` + `resolveDisplayName` layer claim → profile → email local-part. The callback uses them; the `nameToAvatar(userId)` fallback is deleted.
- [x] PBD-04 TopBar falls back to the email address. The Today greeting drops the address when nothing knows a name, rather than greeting a subject id.
- [x] PBD-05 `useNameResolver` batches catalog misses into one `POST /lookup`, caches hits, and asks about an unresolvable id exactly once.
- [x] PBD-06 `UserName` renders "Unknown account" on a settled miss, with the raw id on `title` for forensics and never as the label.

## Track 2 — Bulk access changes, rehearsed

- [x] PBD-07 `services.RehearseBulk` — one directory read shared across the selection; per-person verdict from the same resolver the rest of the product reads.
- [x] PBD-08 Verdicts state the consequence, not just the change: who keeps the role via another source, what a bundle cascades, which grants actually expire.
- [x] PBD-09 Departed accounts are blocked on every additive op; unknown ids are blocked, never silently dropped.
- [x] PBD-10 `extend` touches only grants with a non-null expiry — a permanent grant is never given one.
- [x] PBD-11 `ValidateBulkRequest` rejects unknown ops, missing targets, empty selections, `extend` by zero, and selections over `BulkMaxUsers`.
- [x] PBD-12 `POST /api/v1/grants/bulk` — operator auth, strict JSON, rehearsal by default, `?apply=true` to execute.
- [x] PBD-13 Apply re-rehearses server-side; a client-supplied plan is never executed. Blocked and no-change rows are not acted on.
- [x] PBD-14 Per-person failure isolated, marked `failed` with its cause, and counted separately from successes. Only successful rows recompile their cache.
- [x] PBD-15 Every write routes through `EnqueueDirectGrantPropagation` / the cascade services — no handler-side Zitadel Management call.

## Track 3 — Filters in the URL

- [x] PBD-16 `UserListItem.KeyProjectIDs` — the same projects by id, so a filter link survives a rename.
- [x] PBD-17 `lib/people-filters.ts` — parse/serialize/apply/describe as plain functions over plain data.
- [x] PBD-18 People reads every filter from the URL; the search box debounces into it rather than writing a history entry per keystroke.
- [x] PBD-19 Role membership comes from the role-members endpoint, and rows are left unfiltered while it loads — an empty list would claim nobody holds the role.
- [x] PBD-20 `attention=no-access|departed` express the two cohorts no column exposes. `departed` counts only those still holding roles.
- [x] PBD-21 Bulk mode is URL state (`?bulk=1`), Esc exits, and both leaving the mode and changing the filter drop the selection.
- [x] PBD-22 `GET /api/v1/audit?user_id=` filters on actor OR target at the source; `db.GetAuditLogsForUser` backs it and `GetAuditLogs` delegates.

## Track 4 — Surfaces

- [x] PBD-23 `lib/audit-vocabulary.ts` — one shared vocabulary for the audit log and the person's Activity tab, so the same row cannot read differently on two screens.
- [x] PBD-24 `PersonRequests` — full history including decisions, inline approve/deny, named empty state.
- [x] PBD-25 `PersonActivity` — server-filtered, grouped by day, direction-aware ("by" vs "to"), and explicit when it hits the 200-row cap.
- [x] PBD-26 `/audit?user=` scopes the log with a visible way back out.
- [x] PBD-27 Bundle chips link to the bundle's people; the person header links to their full audit trail and keeps the id last, never as a name.
- [x] PBD-28 Role detail gains one outbound link into pre-armed bulk mode and stays otherwise read-only.
- [x] PBD-29 `components/today/Makerspace.tsx` — gaps, health strip, where access lives, lately. Every number links.
- [x] PBD-30 Today's docstring rewritten to the two-zone contract; the empty queue collapses to one line so the page reads as continuing.

## Review fixes

- [x] PBD-35 `ValidateBulkRequest` rejects a blank or whitespace-only `reason`. The dialog enforced it; the endpoint did not, so a direct caller could move dozens of people's access and leave an audit trail saying nothing about why.
- [x] PBD-36 Activity tab and the audit-trail link are operator-only. The route serves a member their own record, and `/audit` is operator-gated — a member was being offered two controls whose only possible outcome was a 403. Requests stays available to both, because that endpoint accepts self-reads.
- [x] PBD-37 The lookup queue drops settled ids so the next batch proceeds, and answers accumulate across batches. Previously the queue kept settled ids and re-sliced the same first 256 forever; anything past the ceiling never resolved, and the accumulator bug would have discarded earlier batches even once it did.

## Track 5 — One selection model, one rehearsal

- [x] PBD-38 `lib/useRowSelection.ts` — select-all across scope, shift-range from an anchor that resets, paint-from-anchor drag with threshold and no auto-scroll, Space/Shift+Arrow/`a`/`Esc`.
- [x] PBD-39 The drag paints its starting row on the second row it reaches, not on pointerdown, so a press that never moves stays a click — and the trailing click is suppressed so it doesn't undo the first row.
- [x] PBD-40 `SelectionBar` docked at the bottom with a composition line; the triage bar no longer pushes the list down when the first row is ticked.
- [x] PBD-41 `PlanReview` + `RehearsalDialog` extracted; `BulkDialog` reduced to its compose step.
- [x] PBD-42 Drift bulk-attribute and bulk-mark-external rehearse by default, return `BulkPlan`, and report rows somebody else already resolved as no-change rather than rewriting them.
- [x] PBD-43 `POST /api/v1/requests/bulk-decision` — rehearses, then applies through the extracted `resolveOneAccessRequest` so bulk and single decisions cannot diverge. Approving without an attributable reviewer is refused.
- [x] PBD-44 Adopted in People, drift triage, the request queue and Review › Expiring access. Expiring reuses the existing `grants/bulk` extend op — no new endpoint.
- [x] PBD-45 "Select similar" on triage rows selects the cluster (same person or same project).
- [x] PBD-46 Bulk revoke of drift stays absent, and the screen still says so.

## Verification

- [x] PBD-31 `go test ./... && go vet ./...` — backend green.
- [x] PBD-32 `bun run test && bun run lint && bun run build` — UI green (243 tests).
- [ ] PBD-33 **Operator-gated:** sign in against the live Zitadel at `198.51.100.16` and confirm the header and Today greeting render the operator's name. This is the defect that started the change and it cannot be confirmed from a test — the fixture path never had the bug.
- [ ] PBD-34 **Operator-gated:** run one bulk rehearsal against real data and confirm the per-person verdicts match what the individual screens say.
