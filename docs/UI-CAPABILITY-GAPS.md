# MkAuth — UI Capability Gaps

> What the product can do that the console does not let anybody do.
> Audited 2026-08-02 against `465bdb5` (= the revision running on the deployment).
>
> Sources: the rebuild commit `d8676df` and its partial restore `86638b3`; all 88 backend
> routes in [`router.go`](../backend/internal/handlers/router.go) cross-referenced against every
> UI call site; every hook in `ui/src/lib/queries/` cross-referenced against its consumers;
> [`DESIGN-BRIEF.md`](DESIGN-BRIEF.md) §4–§5 per screen; open tasks across 27 OpenSpec changes;
> live container state on the deployment.

## How to read this

Five buckets, ordered by how much they cost you today.

- **A — Regressions.** Worked before the overhaul. Doesn't now, or works worse.
- **B — Built, unreachable.** Backend shipped and tested; no UI reaches it.
- **C — Missing lifecycle.** Gaps in *both* layers. Not a UI errand — the endpoint doesn't exist either.
- **D — Planned, never surfaced.** On the roadmap, no surface at all.
- **E — Live deployment.** What the running instance shows that the console doesn't.

**Bucket A is closed** — all twelve, 2026-08-02. **Buckets B and C's landable items are closed** —
`B1`, `B2`, `C1`, `C2`, `C3`, 2026-08-03. Each item below carries what actually landed and, where
re-verification changed the finding, what was wrong with it.

What is left in C is left for a reason, and the reasons differ: **C6** is real and sized but
unbuilt, **C7** is a product decision rather than an errand, **C9** is half-unbuildable, and
**C4**, **C5**, **C8** are deferred by design.

Work is tracked in [`openspec/changes/ui-capability-gap-closure/`](../openspec/changes/ui-capability-gap-closure/);
its `tasks.md` is the live checklist and mirrors the item ids used here.

---

## A. Regressions from the overhaul

### A1 · Drain outcome hides `errored` and `requeued`

> **Fixed.** `describeDrain()` ([drain-outcome.ts](../ui/src/lib/drain-outcome.ts)) turns a
> `DrainResult` into one line, a tone and an optional detail; `toastDrain()` is what both Resume
> buttons call, so they can no longer describe the same pass differently. Only a pass with nothing
> left to do reads as success. The three halt reasons are three sentences.
>
> Found while verifying: `halted` was a *fifth* silent case, not counted in the original finding —
> `max_retries_exceeded` stops the batch and leaves everything behind that row untouched, and the
> old toast reported that as "N applied, 0 failed".

`DrainResult` carries four numbers. [`pending/page.tsx:54`](../ui/src/app/governance/pending/page.tsx)
toasts only `applied` and `failed`. [`Today.tsx:302`](../ui/src/components/today/Today.tsx) toasts
no numbers at all — just "Queued writes resumed."

`errored` is documented in [`usePropagation.ts:32`](../ui/src/lib/queries/usePropagation.ts) as rows
whose Zitadel outcome was decided but whose state could **not** be persisted. Such a row stays
`in_flight` and needs a second drain. A non-zero value on HTTP 200 means "resume again to retry" —
and nothing on screen says so. `DrainResultBanner` existed for exactly this and was deleted in
`d8676df`.

**Fix:** render the four-number outcome, and say the retry sentence when `errored > 0`.

### A2 · Bundle add-role reverted to two dependent selects

> **Fixed.** [`AddRolesToBundle`](../ui/src/components/bundles/AddRolesToBundle.tsx) — one search
> across every project, grouped, multi-select, roles already in the bundle shown ticked and
> disabled rather than hidden. Apply is sequential (the API takes one role per call, and each add
> cascades over the holders), stops at the first failure, names it, and leaves the rest selected so
> the operator can resume without reconstructing what they asked for.

[`bundles/page.tsx:415`](../ui/src/app/bundles/page.tsx) — choose a project, then choose a role,
one at a time, one round trip each.

`AddRolesToBundlePicker` — searchable, grouped by project, multi-select, sequential apply that
leaves un-applied roles queued so a mid-loop failure is resumable — was deleted and never restored.
Its own header comment named the thing it replaced: "the Stage 3 inline (project, role) Select
pair". That pair is what's back.

### A3 · Event activity lost its status filter

> **Fixed, but not the way the finding proposed.** The backend's `?status=` covers webhook events
> only and takes one exact status, so it can express neither "either table" nor a bucket — and the
> two tables disagree on wording (`processed` vs `completed`). [`outcomeOf`](../ui/src/lib/event-outcome.ts)
> maps both into Done / Waiting / Failed / Not acted on, and the page filters on that. An
> unrecognised status returns itself, so a new backend status can never quietly read as a success
> on a forensic log.

Backend honours `?status=`. [`useWebhookEvents(filter)`](../ui/src/lib/queries/useOperations.ts)
builds the query string. [`operations/page.tsx:36`](../ui/src/app/operations/page.tsx) calls it with
no argument, and offers only a source pill (all / webhook / trigger).

`dropped_enrichment_incomplete` — the status invented in Wave 2 · Part 2 specifically so silent
drops become observable — cannot be filtered for. The one thing the status column was added for is
the one thing you cannot ask the screen.

### A4 · Drift triage lost its filters

> **Fixed, two of three.** Project and source are server-side filters on the queue. `source`
> matters most: a sweep-found row has no actor, so "found by sweep" is "the ones I must judge
> without evidence".
>
> `user_id` is deliberately still not offered. The finding was right that the hook accepts it and
> wrong that the screen wants it — "select everything else for this person" is already on every
> row, works from the row in front of you, and doesn't ask anyone to find a name among three
> hundred.

`useDriftItems` accepts `user_id`, `project_id`, `source`, all backed server-side.
[`UnexplainedAccess.tsx:62`](../ui/src/components/review/UnexplainedAccess.tsx) calls
`useDriftItems()` with nothing, and the screen has no filter controls.

Cluster-select ("select everything else for this user") partially compensates and is arguably the
better affordance for the common case — but it can't answer "show me only what the sweep found" or
"only this project".

### A5 · Bulk confirmation-mode apply is gone entirely

> **Fixed.** Rules are selectable on the index via the shared selection components, with Fire
> immediately / Queue for confirmation in the selection bar. The row click still opens the editor —
> selection lives on the checkbox alone, because unlike the other queues this row's action is
> "open me".

`POST /api/v1/policies/confirmation-mode` has zero callers. `useBulkSetConfirmationMode` has zero
consumers. `ConfirmationModeControls` was deleted.

Design brief §S2 lists this endpoint as part of the Automatic rules screen. Flipping ten rules from
`manual` to `auto` now means ten individual edits.

### A6 · Request duration is hardcoded

> **Fixed, and it was worse than stated.** A member now picks a week, a month, or the end of term,
> with the resolved date shown. The half the finding missed: `duration_days` travels end to end and
> *no operator surface displayed it*, so even a correct ask would have been decided blind. The
> queue, the person's requests tab and the member's own list all state it now — including the case
> that must never be blank, since zero means a grant that never lapses.

[`RequestsScreen.tsx:357`](../ui/src/components/requests/RequestsScreen.tsx) sends
`duration_days: 90` on every request, always. `RequestAccessButton` had a duration picker; the
backend accepts any value. A student asking for one week of laser access is recorded as asking for
a quarter.

### A7 · Member service catalog is gone

> **Fixed, with the data source corrected.** The finding named the `applications` slice of
> `/catalog` as the service catalog. It isn't: `ApplicationCatalog` is a token consumer — a claim
> name and a format — and nobody requests one. What a member asks for is a role in a project.
>
> [`MemberCatalog`](../ui/src/components/member/MemberCatalog.tsx) lists every project and its
> roles under My access, marks the ones already held rather than hiding them, and links each other
> one to the request form with the ask pre-filled.

`MemberAccess` shows held access and one "Request an extension" link. There is no browse-what-exists
surface — no way for a member to see what they *could* ask for.

`/catalog` returns `{users, projects, applications}`. The only consumer,
[`useCatalogUsers`](../ui/src/lib/queries/useCatalogUsers.ts), reads the `users` slice for a persona
dropdown. The applications slice — the actual service catalog — is fetched on every call and read
by nobody.

Related: **D1**, the service→bundle mapping that would make the catalog more than a project/role picker.

### A8 · Audit is pinned at 200 rows

> **Fixed, backend and UI.** Keyset pagination on `(created_at, id)`. The tuple is not
> over-engineering: `created_at` is the *transaction* timestamp, so a cascade writing eight audit
> rows writes eight rows at the identical instant, and a timestamp-only cursor would skip the rest
> of that batch or return it forever. `buildAuditQuery` is split from execution so the placeholder
> arithmetic is tested without a database — a mis-numbered `LIMIT` doesn't error, it returns the
> wrong page. The screen now states which case it is in: more further back, or the end of the log.

The page requests `limit: 200`; the backend caps at 200; there is no offset or cursor and no
"Load more" control. [`useAudit.ts:35`](../ui/src/lib/queries/useAudit.ts) documents a "Load more"
that callers may grow the window with — no caller does, and the backend has nothing to grow into.

Anything older than the last 200 mutations org-wide is unreachable, unless you happen to know whose
`?user=` scope to look under.

### A9 · `project:role` stopped being one thing — **fixed**

> Closed 2026-08-02. Tracked in
> [`ui-capability-gap-closure`](../openspec/changes/ui-capability-gap-closure/) §1; the shape and
> the reasoning are in its [design](../openspec/changes/ui-capability-gap-closure/design.md).
> Re-verifying the claims below turned up three that were overstated — they are marked inline.

The widest of these. The same role key in two projects is two different roles — the design brief
says so in §E2, and [`roles/page.tsx:172`](../ui/src/app/roles/page.tsx) says it in the footnote
under the table. But the product no longer renders that pair consistently anywhere.

**`<RoleName>` has zero call sites.** Before the rebuild it had 13, across audit, bundles, policies,
users, grants and both requests views:

```
fe0a5be:ui/src/app/audit/page.tsx:334
fe0a5be:ui/src/app/bundles/page.tsx:245
fe0a5be:ui/src/app/policies/page.tsx:138,152,159,162
fe0a5be:ui/src/app/users/page.tsx:81,446,584,629
fe0a5be:ui/src/components/grants/GrantsClient.tsx:397
fe0a5be:ui/src/components/requests/AdminRequestsView.tsx:297
fe0a5be:ui/src/components/requests/UserRequestsView.tsx:233
```

The component is still exported from `components/names/index.ts`, still resolves through the shared
name resolver, and is mounted by nothing. Every surface hand-rolls its own role rendering instead,
and they disagree — five formats for one concept:

| Surface | Renders | Project shown? |
|---|---|---|
| [`Today.tsx:182,249`](../ui/src/components/today/Today.tsx) | `<ProjectName /> / <Mono>{role_key}</Mono>` | Yes, raw key |
| [`RequestsScreen.tsx:156`](../ui/src/components/requests/RequestsScreen.tsx) | `<ProjectName /> / <Mono>{role_key}</Mono>` | Yes, raw key |
| [`Makerspace.tsx:238`](../ui/src/components/today/Makerspace.tsx) | `display_label`, falling back to `role_key` | Sometimes |
| [`MemberAccess.tsx:90`](../ui/src/components/member/MemberAccess.tsx) | `humanizeKey(role_key)` | Card heading — *see correction* |
| [`GrantDirectAccess.tsx:100`](../ui/src/components/people/GrantDirectAccess.tsx) | `display_name · role_key` | Implied by a project-scoped select — *not a defect* |
| [`roles/page.tsx:150`](../ui/src/app/roles/page.tsx) | `display_name` + mono key, Project as first column | Yes, properly |
| [`roles/page.tsx:305`](../ui/src/app/roles/page.tsx) (clone-from) | `project_name / role_key` | Yes, properly |

**`display_label` is read on one line in the whole codebase.** The backend composes it at
[`services/roles.go:260`](../backend/internal/services/roles.go) as `"Printing Lab: admin"` —
`advanced-role-crud` built it explicitly as the *global disambiguation label*, its own named
capability (`role-disambiguation`). [`Makerspace.tsx:238`](../ui/src/components/today/Makerspace.tsx)
is the only consumer, and it falls back to the bare key. (This one predates the overhaul — it was
unrendered before `d8676df` too. Long-standing, not a regression.)

**Corrections found on re-verification.** Three of the five "formats" were not defects:

- **`MemberAccess` rows are fine.** Roles are grouped under a card headed by the project name
  ([`MemberAccess.tsx:81`](../ui/src/components/member/MemberAccess.tsx)), so the pair *is*
  established — by structure, which is better than repeating it on every row. The real defect was
  one line lower: the expiry warning at `:107` sits *outside* every card and read "Operator runs
  out on 12 Aug" to a member holding Operator in three projects.
- **The `GrantDirectAccess` role select is fine.** Its sibling select fixes the project. The
  *toast* was the defect — `` `${userName} now holds ${roleKey}` `` fires after the dialog, and its
  project select, are gone.
- **`policies` and `RemovalDialog` are fine.** Both hand-roll `"{Project} / {key}"` — a correct
  pair in the right register. A rule-authoring screen and a destructive confirmation both want the
  exact key. Left alone; uniformity there would cost precision.

**And one the audit missed.** `Makerspace` didn't just consume `display_label` — it rendered
`display_label` ("Printing Lab: admin") in the name slot *and* `project_name` in the trailing
column, printing the project twice on every row.

**What landed.** Two exports, one rule, split by register:

- `<RoleRef projectId roleKey />` — rows. `Printing Lab / trained`: project resolved to its human
  name, role key raw and monospace, because a table is scanned for identifiers. Mounted at all
  nine hand-rolled sites. Renders an em dash if either half is missing — a bare `admin` is worse
  than nothing, because it looks like an answer.
- `roleLabel(projectName, roleKey, displayName?)` — sentences. `Printing Lab / Trained operator`:
  human name, because prose is read and `laser_trained` is not a word. A plain function, not a
  hook, so toasts and ledes can call it.

`<RoleName>` and `formatRoleRef()` — the two dead mechanisms — are gone.

`CatalogRole.display_label` is removed from the backend. It could only ever exist on `/roles`
rows, so the field named as the disambiguation capability could not reach the grant, audit,
request, propagation and drift surfaces where two same-key roles actually collide. The capability
`role-disambiguation` is relocated, not dropped.

### A10 · Role creation has one entry point

> **Fixed.** [`CreateRoleDialog`](../ui/src/components/roles/CreateRoleDialog.tsx) extracted and
> mounted on `/projects/{id}` too, with the project stated rather than offered as a select — the
> dialog was opened from that project's page, so a select there only invites changing it by
> accident.

Create-with-clone works and is correct: [`roles/page.tsx:192`](../ui/src/app/roles/page.tsx) —
key validation, live duplicate check against `(project_id, role_key)`, clone-from listing every
catalog role as `project_name / role_key`, provenance recorded and rendered afterwards as
"cloned from Metal Shop / trained". Restored in `86638b3`. This part is fine.

What's missing is reach. It exists only on `/roles`:

- [`/projects/{id}`](../ui/src/app/projects/[id]/page.tsx) lists that project's roles and offers no
  way to add one — the most natural place to create a role has no affordance.
- The old `CreateRoleModal` supported a pinned-project deep link (`/bundles?createRole=p-1`) so you
  could create a missing role without losing the bundle you were building. Gone.

`/zitadel/projects` has "New role upstream", which is a different and more dangerous action —
it writes to Zitadel without the local row, and the roles index warns about exactly that class of
role.

### A11 · The roles index drops three of its four usage metrics

> **Fixed, and it closes a broken link.** A "Used by" column states what references a role, and
> `is_unused` is both a badge and a filter. Today's "N roles nobody holds" pointed at an unfiltered
> `/roles` and left the reader to find them; it now deep-links `?unused=1`.

`CatalogRole` carries `bundle_count`, `rule_count`, `assigned_user_count` and `is_unused`.
[`roles/page.tsx`](../ui/src/app/roles/page.tsx) renders `assigned_user_count` as the Members
column and ignores the other three.

`is_unused` — zero users, zero bundles, zero rules — is the cleanup signal `advanced-role-crud`
computed the catalog for. A role nobody holds and nothing references is invisible as such.

### A12 · Superseded hooks still in the tree

> **Done.** All six deleted, along with their orphaned types, query keys and one whole file
> (`useDashboard.ts`). `useRecentCascades` going does not settle **B2** — the endpoint it called is
> still live and still has no caller.

Not losses — cleanup. Zero consumers, all replaced: `useZitadelAllGrants` (→ `useUpstreamGrants`,
which pages identically), `useDashboardSummary` (→ `Makerspace`), `useRecentCascades`
(→ `useCascadeGroups`), `useRolesByProject`, `useSimulateMutation` (→ `useTokenSimulator`),
`useWatchlist` (→ `/review/expiring-access`).

---

## B. Backend built, nothing reaches it

### B1 · Shadow Password Vault — no UI, ever

> **Fixed, and not where the brief filed it.** Design brief §S10 puts shadow credentials on
> `System › Hardware sync`. That cannot work: all four endpoints are self-only, so an operator
> standing on a System page could only ever set their own credential.
> [`<ShadowCredential>`](../ui/src/components/member/ShadowCredential.tsx) lives on Member ·
> My access, last on the page — that screen answers "what can I use and why", and a password
> field above somebody's access would make it read as an account screen.
>
> Two sentences on the card are load-bearing and both say what it is *not*: not the institutional
> login, and not read by anything yet. The second is the one the audit's "not blocked by LLDAP"
> framing missed — the vault genuinely works without LLDAP, but nothing *consumes* it, so a member
> who sets a password and tries a door would conclude the product is broken. Saying so costs one
> sentence and means an early adopter is simply ready.
>
> The dialog does not re-implement the complexity rules. `ValidatePasswordComplexity` already
> composes the failing requirements into a sentence, and that sentence is shown verbatim.
>
> **Caught in review:** the console proxy carries its own member allowlist, and the vault's four
> routes were not on it — so the capability was still unreachable, and *invisible*, because the
> card suppressed its own read error. Both fixed. The proxy is the outer of two locks and it
> fails silently; see design Decision 11.

Four user-facing endpoints, none with a caller anywhere in `ui/`:

```
PUT    /api/v1/users/{uid}/shadow-credential
DELETE /api/v1/users/{uid}/shadow-credential
GET    /api/v1/users/{uid}/shadow-credential/status
GET    /api/v1/users/{uid}/shadow-credential/audit
```

Argon2id storage, dedicated `shadow_credential_audit` table, self-only authorization, 23 tests,
migration `000010`. Design brief §S10 names all four. A member has no way to set the credential the
hardware bridge reads.

This is adjacent to the parked LLDAP work but is **not blocked by it**: the vault is self-service
credential storage inside MkAuth and works whether or not LLDAP is reachable. The sync service's
read path (`GET /shadow-credentials/{uid}/hash`, API-key auth) is the part that waits on LLDAP.

### B2 · `GET /api/v1/propagations/cascades` — orphaned

> **Deleted.** Endpoint, handler, `recentCascadesLimit`, `db.GetRecentCascades` and its three
> handler tests. `models.CascadeSummary` stays — it is the per-write shape *inside* a
> `CascadeGroup`, which is where a write is readable: as part of the event that produced it.

The flat recent-cascade feed. `cascade-groups` replaced it on screen (correctly — one entry per
cascade beats one row per write). The endpoint and `useRecentCascades` are now dead. Design brief
§S4 still names `/cascades`.

Expose it or delete it; leaving a live endpoint nothing calls is how the next audit finds it again.

---

## C. Missing lifecycle — both layers

These are not UI errands. The endpoint does not exist either.

### C1 · Mapping rules cannot be deleted

> **Fixed, and it needed no new mechanism.** `DELETE /api/v1/rules/mapping/{id}`.
> `CascadeRuleDeleted` is `CascadeRuleUpdated` with no replacement edge — `rulesAfter` is the
> active set minus this rule — so the revokes fall out of the closure diff the product already
> computes. There is deliberately no coverage check: a role still produced by a bundle, a direct
> grant or another rule simply never leaves the `after` set.
>
> The deletion inherits the rule's own `confirmation_mode`, so a rule that queued its writes
> queues the writes that undo it. On screen the confirmation takes over the editor rather than
> stacking a second dialog on it — the thing you are about to remove is already on screen, and
> the holder count is stated before the click.
>
> Found while verifying: `source_ref` on the outbox rows keeps the deleted rule's id. That column
> is plain text with no foreign key, which is correct — it records what *caused* a write, and the
> cause having since been deleted is exactly what a reader of the change history needs.

No route, no repository function, no UI. `GET`, `POST`, `PUT /rules/mapping/{id}` and
`POST /rules/mapping/validate` — that's the whole surface. A rule authored wrong can be retargeted
forever but never removed.

`feature-coverage.md` claims rule edits "flow through DELETE+CREATE with both halves persisted".
That is stale: `PUT` is the edit path, and `DELETE` has never existed.

### C2 · Bundles cannot be deleted or renamed

> **Fixed, and it turned up a foreign key that would have blocked it.** `PUT /bundles/{id}`
> renames and re-describes — no cascade, no version, because a name is what operators call the
> bundle, not what it grants. `DELETE /bundles/{id}` loops over holders and enqueues each one's
> closure diff in the transaction that deletes the row, because coverage is a property of a
> person: two people holding the same bundle can lose different roles by it going away.
>
> **`onboarding_triggers.bundle_id` referenced `bundles(id)` with no `ON DELETE` clause.** Every
> bundle that had ever onboarded anybody would have been undeletable, and both obvious fixes
> corrupt the log — `CASCADE` deletes evidence somebody was onboarded, `SET NULL` rewrites the row
> to say they were given nothing. Migration `000021` drops the constraint and keeps the column,
> matching `audit_logs.resource_id` and `source_ref`: a log records what happened, it does not
> hold the past open. `<BundleName>` on the event log now reads "a bundle since deleted" there,
> because the default em dash rendered "— assigned automatically".
>
> The welcome flag is reported, never guarded. Refusing to delete the welcome bundle is a rule an
> operator cannot satisfy — the flag clears only by promoting another bundle, and a makerspace
> with one bundle has nothing to promote.

`POST /bundles` creates. Roles can be added and removed, and the welcome flag toggled. There is no
`PUT /bundles/{id}` and no `DELETE /bundles/{id}` — no handler in
[`bundles.go`](../backend/internal/handlers/bundles.go), no `UpdateBundle`/`DeleteBundle` in the
repository layer. A typo in a bundle name is permanent, and a retired bundle stays in the library
forever.

### C3 · Access requests cannot be withdrawn

> **Fixed, and it closed two latent bugs on the way.** `POST /requests/{id}/withdraw`, user-gated
> and self-only in the handler *and* in the `UPDATE`'s `WHERE` clause. Operators are not exempt:
> taking back somebody else's ask is a rejection with the reviewer's name left off.
>
> `withdrawn` is a resolution without a decision — `resolved_at` set, `reviewer_user_id` NULL,
> enforced by the CHECK constraint (migration `000022`) rather than by convention.
>
> The two bugs, both latent until a third terminal state existed:
> - `resolveOneAccessRequest` enumerated the decided statuses (`approved || rejected`) instead of
>   testing `!= "pending"`. A withdrawn request would have stayed decidable, resurrecting an ask
>   its author had taken back.
> - **Both request views read "settled and not approved" as a denial.** The first withdrawal would
>   have shown a member's own retraction back to the operator as a refusal somebody made.
>   `requestOutcome()` now gives one reading in two registers and echoes an unrecognised status
>   back rather than bucketing it — same rule as `outcomeOf` (**A3**), same reason.

No cancel endpoint. A member who asks for the wrong thing must wait for an operator to deny it.
[`requests_bulk.go:82`](../backend/internal/handlers/requests_bulk.go) already writes the copy —
"No such request — it may have been withdrawn." Nothing can withdraw one.

### C4 · Expiring access has no acknowledged/deferred state — **closed, with the reopens-when-the-grant-changes rule**

Design brief §S7 called this out precisely: if operators need to record *"I've seen this and I'm
deliberately letting it go"*, that is a genuine new capability with its own column and endpoint, and
must **not** be faked client-side by hiding rows — a shared queue that diverges per operator is
worse than no queue.

Built as specified, on a decision: **an acknowledgement ends when the grant changes.** Of the three
candidates (grant-changed / timer / manual-only), this is the one that keeps the record honest — an
operator agreed to let a specific role lapse on a specific day, and moving the day means they agreed
to something that no longer exists.

The implementation is the interesting part. The obvious version is a boolean plus something that
clears it — a trigger on `direct_role_grants`, or a check in every write path touching `expires_at`.
Both put the rule somewhere other than where it is read, and both fail *silently*: miss one write
path and an acknowledgement quietly becomes permanent, which is the rule we rejected.

Instead the acknowledgement stores **what was acknowledged** — `acknowledged_expires_at` — and the
read returns it only while that still equals the grant's current `expires_at`. One join condition
*is* the rule. Nothing fires, nothing has to remember, and it is verifiable in a package with no
live database. `direct_role_grants` is UNIQUE on `(user, project, role)` and upserts in place, so an
extension keeps the grant id and moves the date — exactly what the comparison catches.

The write is checked, not trusted: `expires_at` is required on the request and compared under
`FOR UPDATE`, so a page loaded before somebody extended the grant gets `409` and is told to reload.
Storing it anyway would be accepted, never apply, and leave the operator believing they had recorded
something — the precise failure the old "no second button" copy existed to prevent.

Two restraints worth naming, because they cut against the feature's own convenience:

- **Never bulk.** The record's whole value is that a person read the row. "Acknowledge selected"
  would produce the appearance of review and none of it. Bulk *extend* stays — that changes access,
  and reviewing a dozen together is this screen's job.
- **Grouped, not hidden.** Acknowledged rows sit under a counted heading, naming who decided and
  why. Removing them would be the client-side dismissal §S7 forbade, and would hide a decision from
  the person most likely to revise it. Extend stays on those rows; the checkbox does not, so a bulk
  extend cannot undo a decision by accident.

`ON DELETE CASCADE`, reversing the reasoning of `000021`/`000023`: those argued *against* foreign
keys because audit rows are history and must outlive their subject. An acknowledgement is not
history — it annotates a live row, and the grant being swept away on its date is the normal end of
its life. Who decided what stays in `audit_logs`.

See design Decisions 18 and 19.

> **Caught in review — two faults in the extend path around it, both older than C4.**
>
> **The queue's rows are grants; the bulk contract was keyed on people.** The screen bridged that
> by reducing ticked rows to user ids, so ticking "Ada / Laser Lab / trained" extended every
> expiring grant Ada held — other projects, and dates months past the 30 days the screen is scoped
> to. Rows the queue had never rendered. Fixed by carrying `grant_ids` on the contract rather than
> by making the UI select people, which would have fixed the honesty and lost the capability: on a
> per-grant queue, renewing one role and letting another lapse is the ordinary case. `grant_ids` is
> refused on other ops rather than ignored. Design Decision 20.
>
> **Extending left the queue stale.** `POST /users/{id}/grants` upserts on `(user, project, role)`,
> so it is both "grant" and "extend", and its invalidations covered the person but not the queue
> the operator was standing in — the row kept its old date, and an acknowledged row kept showing an
> acknowledgement the new date had already voided. `useApplyBulk` had the same hole, unreported:
> every key root except the screen a bulk extend is launched from. Design Decision 21.

### C5 · Claim profiles are unversioned — **deferred, with its trigger named**

A claim profile edit takes effect on the next token. The audit row says who changed it, never what
it was. An app that breaks after an edit cannot be diffed against the previous shape.

The trigger is concrete: **the first production claim change that needs comparison or reversal.**
When it arrives, the shape to build is an immutable snapshot tied to the application and the audit
event — not a draft/publish lifecycle. Bundles earned theirs because a bundle edit reaches people;
a claim profile edit reaches a token shape, and the question asked of it is "what was it before",
which a snapshot answers and a workflow does not.

### C6 · The audit Trace column is an inference (ISC-44) — **closed**

Confirmed against the code, then fixed. `isCascadeTrace` tested the action prefix and `shortTrace`
rendered `c_XXXX` from `entry.resource_id` — the **bundle or rule id**, not a cascade id — linking
to an unfiltered `/operations/cascades`. Two lies in one column: the identifier was not what its
prefix claimed, and the link did not go to the entry it named.

The fix turned out to be inward rather than outward. `enqueueCascadeRows` already minted one cascade
id per batch and **discarded it**, and all eleven `*AndEnqueue` functions wrote their audit row on
the line immediately above calling it — two halves of one event, written a line apart, with the id
tying them together known only to the second. The audit insert now lives *inside*
`enqueueCascadeRows`: eleven pairs collapsed to eleven calls, the invariant became structural
rather than remembered, and a source-coherence guard fails if a twelfth audit insert ever reappears
in those files.

Then `audit_logs.cascade_id` (migration `000023`, nullable, no foreign key — the outbox is cleared
and an audit row must outlive it), the field on `models.AuditEntry`, and `?cascade=` on Change
history, answered in the query rather than by filtering the fifty-entry glance list.

**Old rows are not backfilled.** Timestamp proximity would be mostly right, and mostly right is the
wrong standard here. They keep the object id they do carry, prefixed `b_` or `R_`, unlinked — the
same identifier as before, minus the prefix that misdescribed it. See design Decisions 12 and 13.

> **Caught in review — the first fix produced a new broken link.** The stamp was gated on "did this
> event cause any writes", when the question is "will those writes appear on the screen this id
> links to". `DeleteDirectGrantAndEnqueue` uses `enqueueCascadeRows` because its ledger delete,
> audit row and outbox rows must commit together — not because it is a cascade — and its writes
> carry `source='direct'`, which Change history excludes. So a direct removal got a trace link to a
> page that filtered out the write it was about, and the empty state told the operator its
> still-pending revoke had been carried out and cleared.
>
> The stamp and the query now read one `cascadeGroupSources` list, passed as a parameter rather
> than spelled out twice inside the SQL. Asserted across the `services`/`db` boundary, because the
> params are built in one and read in the other and the bug lived in the gap — testing either
> package alone passed. Design Decision 16.
>
> **And six actions had no sentence.** `describeAction` falls back to the raw key, which is the
> right failure mode and a silent one: `bundle.updated`, `bundle.deleted`, `mapping_rule.deleted`
> and `access_request.withdrawn` arrived with this change, and `bundle.version_published` /
> `bundle.holder_moved` had been rendering as machine keys since bundle versioning shipped. The map
> is now checked against the Go sources in both directions — an action with no sentence, and copy
> for an action nothing emits. Design Decision 17.

### C7 · App ↔ project cardinality (ISC-45) — **settled: one app, one project**

An application lives in exactly one project, matching Zitadel and the UNIQUE constraint
`app_claim_overrides.application_id` already carries. The design diagram showing Badge Reader
reading four projects was the only thing claiming otherwise, and the diagram is the cheaper of the
two to correct.

The alternative was a feature, not a relaxation: a join table, a rule for competing claim overrides
across projects, a token-audience decision, and a deletion rule for when one of an app's projects is
removed. None of those questions has a caller today.

The apps index keeps warning per project — now correct rather than a workaround. **Reopens on** a
real integration needing roles from two projects in one token. See design Decision 14.

### C8 · Sweep-detected drift cannot name an actor — **deferred, and correct as it stands**

The reconciliation sweep compares grant sets, so it has no event to attribute. "Unknown actor" is
the honest rendering: the sweep is an automated *observation*, and attributing it to a person would
be inventing one.

Closing it means reading Zitadel's event stream for the grant's creation event, at the cost of a
second API path. **The trigger is measured, not felt:** only worth it if operators routinely hit
rows the webhook missed. Nothing counts that today, which is itself the first thing to build if
this becomes a question.

### C9 · The person page in Advanced — **half built, half still not buildable**

Brief §E3: in System, the person page "gains raw grant ids, cascade lineage, hardware sync state —
appended in place, same URL."

**C9a · Raw grant ids — done.** The Advanced panel already showed `direct_role_grants.id`, which
answers "what does MkAuth think" rather than "what does Zitadel hold". It now also shows Zitadel's
own user-grant id, keyed by **project** because that is Zitadel's shape — one grant carries every
role a person holds there, and repeating it per role row would imply otherwise. Operator-only and
not fetched otherwise: this route also serves a member their own record, and the endpoint behind it
is operator-gated. A project with no upstream grant says so and points at Reconciliation, rather
than rendering a dash that reads as "not loaded". See design Decision 15.

**C9b · Hardware sync state — still not buildable.** There is no per-user sync state to render
while the bridge is parked, and a panel that invented one would be exactly the failure
`/system/hardware-sync` was written to avoid. It needs the same contract E1 needs: desired version,
last attempt, result, error, timestamp — per user and per device.

Lineage was already there ([`PersonAccess.tsx`](../ui/src/components/people/PersonAccess.tsx)).
Only two components read `useIsAdvanced` at all — `PersonAccess` and
[`Today.tsx:53`](../ui/src/components/today/Today.tsx) (which swaps its work blocks and gates the
propagation and drift cards). Everywhere else, Advanced only appends nav sections, so in-place
reveal is a two-screen pattern that the brief assumes is general.

---

## D. Planned, never surfaced

| Item | Where it's recorded | State |
|---|---|---|
| **D1** Service→bundle request mapping | `specs/service-catalog`, coverage matrix "Partial" | Requests still resolve to project/role picks |
| **D2** Advanced filters: account age, grant staleness | NEXT.md Phase 5 | People has q / project / role / bundle / attention |
| **D3** Bulk ops idempotent retry | `access-governance` spec | Rehearsal + per-person verdicts landed; retry didn't |
| **D4** Partial failure rollback | NEXT.md Phase 5 | `EnforceMappingRules` logs and continues; a half-applied mapping rule is invisible on every screen |
| **D5** Per-event onboarding policy toggles | `automation-policies` "Partial" | Welcome bundle shipped; the rest didn't |
| **D6** Rate limiting | NEXT.md Phase 5 | Blocked on one decision: in-process vs Redis-backed |
| **D7** Observability — metrics, alert thresholds | `operational-readiness` | Structured logs only |
| **D8** CI/CD | `operational-readiness` | Nothing; every check is run by hand |
| **D9** ⌘K command palette | Struck in the May 2026 audit | Listed for completeness |
| **D10** Google Workspace account poller | design.md §10, Phase 6 | Nothing built. Downstream half (deactivation cascade) is built and tested |
| **D11** LLDAP end-to-end + reconciliation loop | `sync-service` tasks 13.1–13.4 | Parked pending password-propagation research |

---

## E. Live deployment

Deployment at `198.51.100.16` runs `465bdb5` — current with `main`.

**`mkauth_sync` is crash-looping.** Restarts roughly every 60 seconds:

```
[SYNC] MkAuth Sync Service starting...
[SYNC] Configuration error: LLDAP_BIND_PASSWORD is required
```

Expected, given LLDAP is parked. The gap is that nothing in the console shows it.
[`/system/hardware-sync`](../ui/src/app/system/hardware-sync/page.tsx) states "not connected yet"
as a static fact written into the page. A sync container that is deliberately parked and one that is
genuinely broken render identically — which is the same failure mode the page's own header comment
argues against for empty tables.

---

## New findings, surfaced while closing bucket A

Not in the original audit. None are blocking; recorded so they are not rediscovered.

- **`/webhook/events` and `/onboarding/triggers` are unbounded.** Neither query has a `LIMIT`;
  both return the whole table on a five-second poll. Fine at makerspace scale, a cliff at any
  other. The audit log's fix (**A8**) is the shape this wants.
- **A member's own request list names the project but not the role.** [`RequestsScreen`](../ui/src/components/requests/RequestsScreen.tsx)
  member rows render `<ProjectName>` alone, so two asks for the same space read identically. Fixing
  it properly needs a member-register role component — `<RoleRef>` shows the raw key, which the
  member view deliberately never does. Same shape as **A9**, different register.
- **`useWebhookEvents`'s `status` param now has no caller** (see **A3**), and the backend's
  `?status=` has none either. Same class as **B2** — expose or delete, don't leave it.
- **The triage tab badge counts the filtered queue.** With a project filter applied, "Triage queue
  3" is three *in that project*. The header meta says "matching these filters" so it is not silent,
  but the honest fix is to read the unfiltered depth from the governance summary.

## Doc drift found on the way

- `feature-coverage.md` — "rule edits flow through DELETE+CREATE". No `DELETE` exists (**C1**).
- `feature-coverage.md` — UI style row reads "explicit theme toggle / mode persistence not
  evidenced". Both exist: [`shell/ThemeToggle.tsx`](../ui/src/components/shell/ThemeToggle.tsx).
- `feature-coverage.md` — audit row reads "actor identity is currently demo/static in UI flows".
  Resolved through the name resolver since the live-directory work.
- `ROADMAP.md` Phase 5 — Welcome Bundle Configuration listed open; it shipped
  (migration `000012`, `PUT /bundles/{id}/welcome`, UI on the bundles page). Already noted in
  `NEXT.md`.

## Verified as *not* lost

Checked and present, so nobody re-audits them: create bundle · bundle impact preview ·
welcome-bundle toggle · remove role from bundle · create/retarget mapping rule with
validate-before-save and cycle detection · per-rule confirmation mode · global confirmation-mode
default · role creation with clone-from and provenance · role → members · direct grant create and
delete · bulk grants with rehearsal · bulk request decisions with rehearsal · drift attribute /
revoke / mark-external and both bulk resolutions · reconcile-now · reconciliation diff · audit CSV
export and per-person scoping · claim profile editor and token simulator · app claim override
create and delete · upstream console (users, projects, roles, grants) behind its consequence
disclosure · Zitadel health as a sentence · signing-key rotation status · access map with root view
· degraded-directory banner · drift chime and its toggle · server-side people search ·
People URL filters and bulk mode · Today + Makerspace.
