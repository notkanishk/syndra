> **Status:** bundle-lifecycle-repair — tasks | [< Index](../../INDEX.md) · [Proposal](proposal.md) · [Design](design.md)

# Bundle Lifecycle Repair — tasks

## 1. A bundle is created with its roles ✅

- [x] 1.1 `db.CreateBundle` takes `roles []models.BundleRole` and writes the working copy and v1 from that one slice, in the existing transaction. A v1 disagreeing with `bundle_roles` by one row is an unpublished change nobody made
- [x] 1.2 `db.InitialVersionNote` — "Created with N roles.", keeping "Created empty." reachable for the bundles that already carry it
- [x] 1.3 `CreateBundleRequest.Roles`, required; `normalizeNewBundleRoles` trims, de-duplicates on (project_id, role_key), and refuses a half-named role. Validation runs before the write
- [x] 1.4 The create dialog asks for them. `RolePicker` extracted from `AddRolesToBundle` and shared, rather than a second picker
- [x] 1.5 `FieldLabel` takes an `id`, so a field that is a group of controls can be labelled by it
- [x] 1.6 Tests: no roles is a 400 naming `roles` and reaches no write; the roles reach the write trimmed and deduped; a half-named role is refused; `InitialVersionNote` shapes; the dialog refuses with a stated reason and sends what was ticked

## 2. What a bundle grants, as a question a caller can ask ✅

- [x] 2.1 `GET /bundles/{id}/roles?published=true` answers from `db.LatestVersionRoles`; the default still answers the working copy
- [x] 2.2 Both branches normalise nil to `[]` — the clients call `.length` and `.map` on this payload
- [x] 2.3 `useBundleRoles(id, {published})` and `useBundleRolesByBundle(ids, {published})`, cached under separate keys so the two questions cannot share an answer
- [x] 2.4 `ManageBundles` reads the published version for the role count AND the grant preview
- [x] 2.5 It states when the bundle has unpublished edits that are not part of the assignment, and which version an assignment gives
- [x] 2.6 Tests: `?published=true` routes to the version and the default to the working copy (the fifth instance of this bug class, guarded at the boundary); empty is `[]` not `null`; the panel asks for the published set, previews it, and names the unpublished remainder

## 3. Publishing and moving holders cite a durable approval ✅

- [x] 3.1 `BulkPlan.RequestFingerprint` (not serialised) — for the surfaces whose cohort is derived from the world rather than named in the body
- [x] 3.2 `FingerprintBundlePublish` binds the bundle, `migrate`, the version contents and the holder set with each holder's version
- [x] 3.3 `FingerprintMoveHolders` binds the bundle, the target version, its contents and the named people
- [x] 3.4 Per-outcome fingerprints on every row both rehearsals emit, blocked and no-change included — a blocked row re-verified is meaningful in its own right
- [x] 3.5 `planSurfaceBundlePublish` / `planSurfaceBundleMove`; both rehearsals call `issuePlan`, both applies claim it
- [x] 3.6 A publish whose rehearsal reached nobody issues no approval and needs none — the shape `issueRollbackPlan` already uses
- [x] 3.7 `acknowledge_scope` on both rehearsals, so the blast-radius ceremony applies here at the same threshold as everywhere else
- [x] 3.8 Both UI hooks send the citation; both dialogs take it from the plan on screen
- [x] 3.9 Tests: the rehearsal returns an id; an apply without one writes nothing; a cited apply publishes; a holder whose delta moved is refused; a cohort that grew is `PLAN_REQUEST_MISMATCH` and writes nothing; a publish approval is not citable on the move surface; fingerprint properties (read-order blind, cohort growth, holder version, contents-without-movement, `migrate`)

## 4. A blocked control says why ✅

- [x] 4.1 `applyBlocked` — one string; `disabled` derived from it so state and explanation cannot diverge
- [x] 4.2 `definitionLabel` gated on "no row will act" rather than "there are no rows"; the citation requirement moved onto whether the plan HAS rows, which is what `issuePlan` derives it from
- [x] 4.3 An incoherent plan — no `outcomes`, or an empty list with a non-zero total — stays refused, and says so
- [x] 4.4 `notReadyReason` on the compose step
- [x] 4.5 `PublishVersionDialog` passes the label, the reason and the citation; `MoveHoldersDialog` likewise — a move whose rows all read "no change to their access" still moves the pin, which is what clears the stale-holder count
- [x] 4.6 Tests: each blocked route renders its reason and none appears when Apply is available; forty unmoved people take the definition label and cite their approval; rows without an approval stay refused; a publish with no holders and a publish leaving holders behind both reach an enabled Apply

## 5. Verification

- [x] 5.1 `go test ./... && go vet ./...` in `backend/`
- [x] 5.2 `bun run test && bun run lint && bun run build` in `ui/`
- [x] 5.3 **Walked on the dev deployment, 2026-09-09.** Database dumped first
  (`backups/pre-bundle-lifecycle-20260909-160520.sql.gz`). The box has no
  Zitadel, so `SYNDRA_API_KEY` is the operator credential and the outbox stays
  queued — which is orthogonal to everything here, since all of it is
  Syndra-side state and the plan gate.

  Through the API, against real Postgres:
  - creating with no roles → `400` `{"roles":"at least one"}`, no bundle written
  - creating with two → v1 noted `Created with 2 roles.`, `bundle_version_roles`
    = 2, `bundle_roles` = 2, and the draft reports **zero** unpublished changes.
    **This is the pair §4b listed as unproven**, and the defect that made a new
    bundle report changes nobody had made
  - a working-copy edit leaves `?published=true` on v1's roles
  - publishing with no holders: applied with no citation, v2 written
  - assigning pinned v2 and projected v2's three roles
  - publishing with a holder: the rehearsal **carries a `plan_id`** and the row
    reads `v2 → v3, gains admin-staff-x` — the thing that was always absent
  - applying with no citation → `400 PLAN_REQUIRED`, nothing written
  - editing the working copy under a live approval → `409
    PLAN_REQUEST_MISMATCH`, nothing written. The request fingerprint working
  - applying the citation → v3 written, the holder repinned v2 → v3
  - publishing while leaving the holder behind → applied, `apply: 0`, holder
    still on v3
  - an empty draft with a valid citation → `409 NOTHING_TO_PUBLISH`
  - moving holders: `plan_id` issued, `400` without it, applied with it
  - a publish approval cited on the move endpoint → refused

  Through a browser (the demo operator identity, Chrome against `:3001`), the
  four reported symptoms specifically:
  - the create dialog carries the role picker; Create is disabled reading
    "A bundle needs a name.", then "Pick at least one role…", then enables as
    "Create with 2 roles"
  - a brand-new bundle shows **no** unpublished-changes strip
  - `Publish v2` on a bundle nobody holds reaches an **enabled** `Publish v2`
  - `Preview the change` is disabled *and says why* while the migrate question
    is unanswered, then enables
  - a publish reaching a holder reaches an **enabled** `Apply to 1 person`, row
    `v3 → v4, gains budget_approver`, and applies
  - the assign panel lists the four **published** roles, not the fifth
    unpublished one, counts "4 roles", and states "has 1 unpublished change.
    Those are not part of this — an assignment gives v3…"

  Everything created was removed afterwards and the deployment is back to its
  prior state: one bundle, no assignments, no orphaned outbox rows, no leftover
  approvals.

- [x] 5.4 Three defects the browser found that the guards could not — all on the
  result step, which no operator could reach until publishing was appliable.
  See §7.

## 7. What the deployment found ✅

Four defects that no test in this repo would have caught, because every one of
them lives past a step no operator could reach. Making publishing appliable is
what exposed them — which is the argument for walking a sequence live rather
than only asserting its parts.

- [x] 7.1 **`MoveHolders` reported nothing it did.** `plan, err :=` inside the
  locked closure declared a second `plan`, leaving the returned one zero-valued:
  a move that repinned somebody answered `op: ""`, `outcomes: null`, every count
  nought, and the result step renders from exactly that. Neither the compiler
  nor `go vet` objects — shadowing is legal and `err` is used.
  `PublishBundleVersion` has the same shape and got it right. Regression test
  fails on the shadow.
- [x] 7.2 **The publish dialog named the wrong version on the result step.** A
  successful publish invalidates the draft query, so the `draft` prop moves the
  instant the write lands: the step reported "Publish … v3" for the publish that
  had just created v2. The dialog now holds the draft it opened on.
- [x] 7.3 **"Applied to 0 people."** was the report for publishing a version
  nothing holds, and for saving a mapping before anybody has the role — true,
  and reading as a failure, when both are the act succeeding. Now reports the
  act. Guarded on every count being nought, so twelve people waiting in the
  outbox keep their counts.
- [x] 7.4 **The empty-cohort sentence said "this role"** on a bundle screen.
  Read from the plan's `op` instead, which already knows.
- [x] 7.5 **"the 1 person who already hold it"** — my own copy, from
  pluralising the noun and not the verb. Test covers the one-holder case, which
  is the common case in a makerspace this size.

## 8. Sweep for the same classes elsewhere ✅

Asked for after the deployment walk: are any of these defects repeated?

- [x] 8.1 **Shadowed outer variable — none elsewhere.** Checked three ways:
  x/tools' shadow analyzer (validated against a reconstruction of the real
  bug), a broader AST pass covering what that analyzer misses (a shadowed
  *named result* returned by a bare `return` is NOT reported by it), and manual
  inspection of every callback site of the wrapper kind. Only `err` shadows
  remain, which are idiomatic.
- [x] 8.2 **A guard for it**, `repoguard.TestAClosureDoesNotShadowWhatItsCallerReads`.
  Narrowed to the dangerous shape — a non-error function-scope value or named
  result re-declared inside a closure — because the off-the-shelf analyzer
  reports forty idiomatic `err` shadows alongside, which is why nobody runs it.
  Zero findings as the tree stands; fails on the real defect when it is put
  back, which is the only evidence a guard is worth having.
- [x] 8.3 **All eleven plan-gated surfaces audited** — every rehearsal issues an
  approval and every apply claims one. No surface is a dead end and none has a
  decorative approval.
- [x] 8.4 **Why it recurred: the surface list lived in three files.** Six
  constants in `plan_gate.go`, four in `mapping_plan.go`, one in
  `entitlements.go` — so reading the file where the mechanism lives gave a
  confident wrong answer omitting five surfaces, `mappings.rollback` among them,
  which was the first instance of this defect. Consolidated into `plan_gate.go`
  with the reason recorded there. This is the actual root cause of the
  recurrence, and it was organisational rather than logical.
- [x] 8.5 **`claimPublishPlan` checked emptiness before the citation.** A publish
  rehearsed against holders who were all unassigned before the apply took the
  no-citation path: the approval went unspent and citable until expiry, and the
  publish proceeded without checking the version contents the operator had
  reviewed. It could not move the wrong person — nobody is left — but it could
  publish a version other than the approved one. A caller holding a citation now
  always presents it; only a caller with none falls through to the definition
  path. Test covers it.

## 9. The UI classes, swept ✅

The same four defects the deployment walk found, looked for everywhere else.

- [x] 9.1 **`PlanReview` reported a missing list of "people"** to every caller,
  including the screens driven with `["account", "accounts"]`,
  `["item", "items"]` and `["request", "requests"]` — one branch above the
  sentence I had just fixed. It now takes the noun the dialog already knows.
- [x] 9.2 **The empty-cohort sentence fell through to "role"** for
  `rollback_mappings`, which restores a whole target's set across several roles.
  Now exhaustive over the op union, so the next op added fails the typecheck
  rather than inheriting somebody else's sentence.

  The obvious correction was wrong and a test caught it: a mapping is not
  something anybody HOLDS — it hangs off a (project, role) pair and describes
  what that role reaches — so the mapping surfaces genuinely do say "role".
- [x] 9.3 **`queuedNote` named the wrong drain rule for `delete_mapping`.**
  Deleting a mapping only takes access away, so its rows drain on the background
  runner; the operator was told to go and send them from Pending changes, a
  queue that would empty on its own before they arrived. `rollback_mappings` is
  deliberately left out — it restores a set, so it both grants and revokes, and
  the safe direction is the copy that sends somebody to look.
- [x] 9.4 **Four more plural/verb disagreements**, all of the shape I shipped:
  "the 1 person who hold X keep exactly what they have", "the 1 person already
  holding it come along", "the 1 person holding it lose whatever…", and "The 1
  who already hold it" as both a field label and a radiogroup's accessible name.
  The publish dialog's phrase is written once now and used in all three places.
- [x] 9.5 **`DeleteBundleDialog` was destroyed by its own mutation.** The page
  looked the open bundle up in the live list on every render, and the delete
  invalidates that list — so the deleted row vanished and the workspace owning
  the dialog either unmounted or remounted on a different bundle. The dialog
  stays open on purpose to report how many revocations were queued, and the
  previous commit here had added a comment saying that clearing the selection
  early throws away an outcome nobody has read. The refetch was doing it anyway
  and Done was unreachable. Same class as 7.2, one component over. Test fails
  without the fix.
- [x] 9.6 Not taken, recorded in NEXT.md §4b: seven more disabled controls that
  give no reason (the four where an operator gets genuinely stuck are fixed),
  and the pre-existing button remount when a `reason` clears.

## 6. Follow-ups

- [ ] 6.1 `bundle-versioning` §6.1–6.3 remain open and untouched here (estate-wide catch-up, stale counts on Today, hand-picked `MoveHolders` subsets)
- [ ] 6.2 `definitionLabel` now covers a definition being saved and a version being cut. Rename when something else touches those four mapping call sites
- [ ] 6.3 The pre-existing `tsc --noEmit` failures in `ui/src/**/__tests__` (missing `holds_due`, missing `unpublished_changes`, `downlevelIteration`) are unrelated to this change and are not covered by `bun run build`, which typechecks only the app. Worth a pass of its own
