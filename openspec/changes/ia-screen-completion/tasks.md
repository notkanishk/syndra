# Tasks — completing the drawn IA

## Track 1 — Data plane

- [x] ISC-01 Migration `000019`: `pending_zitadel_propagations.cascade_id` + partial index; `drift_items.{upstream_actor, upstream_created_at, last_seen_at}`. Down migration drops all four.
- [x] ISC-02 `enqueueCascadeRows` stamps one cascade id per call. Every propagation read carries it; readers fall back to the row id for pre-migration rows.
- [x] ISC-03 `UpsertDriftItemWithEvidence`: refreshes `last_seen_at` on re-detection, `COALESCE`s evidence so a sweep cannot erase a webhook's actor. `UpsertDriftItem` delegates.
- [x] ISC-04 `WebhookPayload.{EditorID, EventCreatedAt}` carried from Zitadel's `userID` / `created_at`; `detectWebhookDrift` passes them as evidence.
- [x] ISC-05 `db.GetCascadeGroups` — cascade-grouped history including unlanded writes.
- [x] ISC-06 `db.GetBundleHolderCounts` — one query, not one per bundle.
- [x] ISC-07 `models.ProjectRole.Group` becomes its own field; the Zitadel directory stops writing the group into `Description`.

## Track 2 — Control plane

- [x] ISC-08 `GET /review/expiring-grants?within_days=` (default 30, capped at 365). Closes handoff gap 5.
- [x] ISC-09 `GET /propagations/cascade-groups`.
- [x] ISC-10 `POST /governance/drift/bulk-mark-external`. Bulk revoke stays absent by design.
- [x] ISC-11 `GET /governance/drift` unfiltered returns the enriched, risk-then-age-ordered triage view; filtered requests keep the raw listing.
- [x] ISC-12 `GET /users` gains bundle names, project count and the needs-attention trio; search matches role keys.
- [x] ISC-13 `GET /roles` carries `group` and clone provenance; `GET /rules/mapping` and `GET /bundles` carry holder counts.

## Track 3 — Screens the revised handoff draws

- [x] ISC-14 People index (§05): Needs-attention column, bundle pills, "N roles across M projects", project filter, explicit pagination, departed rows at reduced contrast.
- [x] ISC-15 S6 Unexplained access: bulk bar with the stated absence, expanded evidence row, risk pills in words, fixed resolution order, revoke dialog with consequence and the person's other items, reconciliation with both drift directions and agreement stated.
- [x] ISC-16 S7 Expiring access: own endpoint and window, Granted column, only the soonest row emphasised, the two explanatory cards.
- [x] ISC-17 S1 Bundles: three columns, role removal with the impact panel in the space a dialog would have taken, holder counts, default-bundle copy.
- [x] ISC-18 S2 Automatic rules: names not ids, holders, rule editor with validation that names the chain, save blocked until validation passes, per-rule confirmation mode.
- [x] ISC-19 S2b Automation settings: cost-described options and the org-wide placement note.
- [x] ISC-20 S3 Pending changes: cascade grouping with the shared-cascade line, amber unreachable banner, Caused-by column.
- [x] ISC-21 S4 Change history: one entry per cascade, three-word state vocabulary, consequence sentence, unlanded cascades included.
- [x] ISC-22 S8 Audit: sentences not verb keys, destructive verb coloured alone, date range, actor filter, CSV export with names resolved, trace column.
- [x] ISC-23 S9 Identity provider: health as a sentence with a cause, three stat cards, upstream inspection disabled with a visible reason when unreachable.
- [x] ISC-24 S10 Hardware sync: dashed parked panel; the intents table renders only when it holds rows.
- [x] ISC-25 S11 Event activity: webhook events and onboarding triggers merged into one time-ordered stream, names resolved, payload drilldown, only the error row tinted.
- [x] ISC-26 Apps index: Kind column, project names, mixed-format warning.
- [x] ISC-27 Project detail: descriptions in full, group, clone provenance, Token format jump.
- [x] ISC-28 Roles index: group column and filter, clone provenance, honest partial-coverage notice.

## Track 4 — Restored capability

- [x] ISC-29 Create role, including clone-from (`POST /roles` had no caller).
- [x] ISC-30 Remove a role from a bundle (`DELETE /bundles/{id}/roles/…` had no caller).
- [x] ISC-31 Bulk adopt / bulk mark-as-external in triage.
- [x] ISC-32 Rule editing and per-rule confirmation mode (`PUT /rules/mapping/{id}`, `POST /policies/confirmation-mode` had no callers).
- [x] ISC-33 Upstream console restored at `/zitadel/{projects,users,grants}`: reads plus the write escape hatches, collapsed behind a disclosure that names the three consequences.
- [x] ISC-34 Audit CSV export; event payload inspection.

## Track 5 — Names, the map, and demo data

- [x] ISC-35 Every remaining raw project id replaced by a resolved name: rules, event activity, hardware sync, apps index, bundle roles, bundle cascade preview, app token header.
- [x] ISC-36 `UserAvatar` resolves initials from the id, so a row no longer pairs a resolved name with a blank disc.
- [x] ISC-37 Access map opens on a browsable root grouped by kind, with a breadcrumb back; `SHOW` becomes a filter tracking hidden kinds.
- [x] ISC-38 Degraded banner covers `seed_active` alongside a live directory — the one case where demo data was visible with no signal at all.

## Track 6 — Verification

- [x] ISC-39 Backend: migration-coherence guard for `000019`; triage ranking, service-account detection and other-items counting; People-index attention routing and role-key search.
- [x] ISC-40 Frontend: access-map root and return path; triage order, stated absence, evidence, service-account neutralisation; People-index signal precedence and departed contrast; cascade grouping and the disabled-confirm reason; degraded-vs-seeded banner.
- [x] ISC-41 `go test ./... && go vet ./...`, `bun run test && bun run lint && bun run build` all green.

## Review fixes

- [x] ISC-46 **P1** — drift attribution accepted `source: "rule"` with no rule id and never checked that any rule existed, and `bundle` accepted an empty reference. First pass added validation that the named bundle/rule could produce the role. That was not enough, and ISC-48 finished it.
- [x] ISC-48 **P1** — validating that a bundle or rule *could* produce the role still left the write creating a `direct_role_grants` row and nothing else: no bundle assignment, no rule-derived relationship. Since cascades deliberately never touch the ledger, `direct_role_grants.source ∈ {bundle, rule}` could only ever have come from this endpoint — a label with nothing behind it, on access that then survived removal of the bundle it named. Attribution is now `external_backfill` only, with `source_ref` gone from the contract and the rejection explaining itself. Implementing real ownership was rejected as worse than the bug: assigning a bundle to explain one role hands over every other role it carries, and making a rule produce the role means granting the person the rule's frequently safety-gated input. Five tests, mutation-checked against the source gate; the bundle-remap validation and its dead dependency deleted with the feature they guarded.
- [x] ISC-47 **P2** — both bulk endpoints returned success/failure counts that the UI discarded, always toasting the originally selected count and clearing the whole selection. A concurrent resolution or a failed write read as complete success, leaving unexplained access reported as handled. The endpoints now return `failed_ids`, and the screen reports the server's counts and retains exactly the failed rows. Three backend tests, four UI tests, mutation-checked against the old toast.

## Open

- [ ] ISC-42 Exercise `000019`, the enriched drift read and the upstream console against a live Postgres + Redis + Zitadel. Unit-tested only so far.
- [ ] ISC-43 Per-route manual a11y checklist for the twelve screens this change touched (empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast).
- [ ] ISC-44 Thread a cascade id through audit writes so the Trace column is a real trace rather than an inference from the action name.
- [ ] ISC-45 Decide whether an application may read more than one project. The design assumes it can; the data model says one. Until then the apps index warns per project rather than per app.
