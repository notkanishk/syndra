# Tasks — Basic / Advanced IA + real claim shaping

Task IDs prefixed `BIA-`. All complete unless marked otherwise.

## Track 1 — Claim shaping (the data plane)

- [x] BIA-01 `internal/claims` package: `Profile`, `Facts`, `FormatRoles`, `Emit`, `Shape`, `Keys`, `ValidateProfile`, `Conflicts`. No DB or network, so the data plane can shape inside its Redis budget.
- [x] BIA-02 Migration `000018_claim_shaping`: `attribute_claims` + `static_claims` on `claim_profiles`; `app_claim_overrides` table with a per-project unique claim key.
- [x] BIA-03 `db/claim_profiles.go`: full CRUD for both, JSONB round-trip.
- [x] BIA-04 Compiler writes facts, not a claim map; captures profile attributes via the new injectable `cacheFindUser`.
- [x] BIA-05 `handlers/action.go` shapes on read; `claim_shape:<projectID>` read-through cache; namespacing reduced to a collision guard for unconfigured defaults.
- [x] BIA-06 `SimulateApplication` calls the same shaper; adds `owned_claims` + `claim_owners` so a sibling app's key is attributable.
- [x] BIA-07 `services/claim_shape.go`: resolve, operator view, save with cross-project key validation.
- [x] BIA-08 Endpoints: `GET /projects/{id}/claim-shape`, `PUT /projects/{id}/claim-profile`, `PUT|DELETE /applications/{id}/claim-profile`, `GET /claim-attributes`. Every write invalidates the cached shape.
- [x] BIA-09 Tests: `claims` package (format, emit, omission of unknown attributes, validation, conflicts); services (fallback, override scoping, cross-project collision, project derived from the app); handlers (configured name/format applied, attribute + static emitted, default AND override both present, per-project keys kept, collision namespaced); migration coherence.

## Track 2 — The three missing endpoints

- [x] BIA-10 `GET /governance/indicators` — four scalars, shared expiry horizon with Today.
- [x] BIA-11 `GET /projects/{id}/roles/{key}/members` — sources in fixed order, grant id on direct rows.
- [x] BIA-12 `DELETE /users/{id}/grants/{grantId}` — ledger delete + audit + effective-access delta in one transaction, `ErrGrantNotFound` → 404, cache rebuilt before responding.
- [x] BIA-13 Tests for all three, including the 404-on-already-removed race and the cache-rebuild contract.

## Track 3 — Design system and shell

- [x] BIA-14 `globals.css` rewritten: both themes in full, semantic roles, geometry, type roles, `--canvas` for the content column.
- [x] BIA-15 Bricolage Grotesque / Figtree / JetBrains Mono via `next/font`; pre-paint theme script.
- [x] BIA-16 `lib/nav.ts` — the navigation contract, with `pattern` matching so a role detail is not a project detail.
- [x] BIA-17 Rail, top bar, view switch, theme toggle, `AppShell`, `PageCrumbProvider`.
- [x] BIA-18 `UiViewProvider` with `revealInAdvanced` scoped-jump behaviour.
- [x] BIA-19 Primitives rebuilt: Button (danger is an outline; `dangerConfirm` only inside a dialog; `reason` renders visibly), Card, Badge, Modal, Input, Select, Segmented, FilterPills, Avatar, PageHeader.
- [x] BIA-20 Four states: `RowSkeleton`, `EmptyState`, `ErrorState`, `DegradedBanner`, and `ListStates` so no view can silently skip one.
- [x] BIA-21 Access source component — three kinds, fixed order, multiples collapsed, popover.

## Track 4 — Screens

- [x] BIA-22 Today (Basic two blocks, Advanced appends two), member My access.
- [x] BIA-23 People index, person detail with in-place Advanced lineage, Manage bundles, Grant direct access.
- [x] BIA-24 Source-specific removal dialogs (direct / bundle / automatic) with residual outcomes.
- [x] BIA-25 Projects, project detail, roles index with the required partial-coverage notice, role → members.
- [x] BIA-26 Apps index and the token screen: format editor + preview.
- [x] BIA-27 Requests (operator queue + member self-service).
- [x] BIA-28 Advanced: Bundles, Automatic rules, Automation settings, Pending changes, Change history, Access map, Unexplained access (triage + reconciliation), Expiring access, Audit, Identity provider, Hardware sync, Event activity.
- [x] BIA-29 `/grants` 301 → `/governance/drift?tab=reconciliation`; retired components deleted.
- [x] BIA-30 `middleware.ts` member allowlist.

## Review fixes

- [x] BIA-38 **P1** — the direct-grant delete enqueued an unconditional revoke, so removing a grant for a role also held via a bundle or rule removed it upstream despite the dialog promising retention. Now computes the closure delta (`userBaseHoldingsExcludingGrant`, excluding by grant id) and enqueues only genuine losses; reports `revoked_roles` / `retained_roles`. Six tests, mutation-checked against the old behaviour.
- [x] BIA-40 **P2** — three navigations wrapped `<Button>` in a `<Link>`, rendering `<a><button/></a>`: invalid HTML, two overlapping controls for a keyboard or screen-reader user. Added `ButtonLink`, which shares the styling and nothing else; the three sites converted. Canary + component tests, mutation-checked.
- [x] BIA-39 **P2** — the reconciliation header interpolated a `<Relative/>` component into a template literal, rendering `generated $<Relative … />` as text. Fixed, plus a canary that fails on the stranded `$<` signature anywhere in source.
- [x] BIA-41 **P1** — BIA-30's allowlist was a second copy of the member routes, and `addon-platform` 18.1 added `Network storage` to only one of them: every member who tapped their own storage row was 307'd back to `/`. The rail offered a destination middleware refused, which is the exact failure this change's one-file rule exists to prevent, and it survived because the only member test asserted what a member could *not* reach. `middleware.ts` now reads `memberMayVisit` from `nav.ts`; `/login` loses its seat, since a valid session is redirected off it by an earlier guard and an absent one never reaches the member check. `memberMayVisit` gained sub-path matching on the `leafMatches` rule, so `/storage/{target}` is reachable the day that route exists rather than reproducing this bug one segment deeper — `/` stays exact or it admits everything, and the boundary is a `/` so `/storage-admin` is still refused. Tests: a member reaches every row `MEMBER_NAV` renders (iterated, not a literal list, which is how the first one drifted), a child of a member route, and the prefix trap; plus a source guard that fails if middleware names a member route itself, because a second list that agrees today passes every behavioural test and drifts tomorrow. Mutation-checked in both directions — restoring the local list kills three tests, dropping sub-path matching kills two.

## Track 5 — Verification

- [x] BIA-31 Frontend tests: nav contract, rail stability, access source, removal dialogs, token format editor.
- [x] BIA-32 `design-system.test.ts` replaces `no-legacy-tokens.test.ts` — bans the retired palette, hardcoded design-board hex, and removed focus outlines.
- [x] BIA-33 `go test ./... && go vet ./...`, `bun run test && bun run lint && bun run build` all green.
- [x] BIA-34 Browser pass: both themes, both rails, breadcrumbs, error state. One real hydration mismatch found and fixed (locale-dependent dates).

## Open

- [ ] BIA-35 Exercise the claim editor, the simulator, role → members and the direct-grant delete against a live Postgres + Redis + Zitadel. Unit-tested only so far.
- [ ] BIA-36 Per-route manual a11y checklist (empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast).
- [ ] BIA-37 Legacy path alignment (`/policies` → `/automation/rules`, `/graph` → `/automation/access-map`, `/operations` → `/system/events`) behind redirects. Cosmetic; deliberately deferred.
