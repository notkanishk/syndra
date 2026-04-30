# Tasks

Tasks are grouped by stage. Stage 1 must complete before Stage 2; Stage 2 and Stage 3 can interleave by page; Stage 4 can begin once Stage 1 is in. Task IDs prefixed `OCR-S{n}-{nn}`.

## Stage 1 — Foundation

### 1.A OpenSpec scaffolding

- [x] OCR-S1-01 Create `openspec/changes/obsidian-clarity-redesign/{proposal,design,tasks}.md` and initial spec deltas under `specs/{operational-readiness,user-management,access-governance,application-claims}/spec.md`.

### 1.B Backend — lookup endpoint

- [x] OCR-S1-02 New `backend/internal/handlers/lookup.go` — `handleLookup` for `POST /api/v1/lookup`. Decode via `decodeJSONStrict` into `LookupRequest{ UserIDs, ProjectIDs, RoleKeys [{ProjectID,RoleKey}], BundleIDs }`. Cap each array at 256 → 400 on overflow. Tolerate partial misses (missing IDs absent from the response, never a 404).
- [x] OCR-S1-03 Wire route in `backend/internal/handlers/router.go` under `withCORS(withUserAuth(...))`.
- [x] OCR-S1-04 New `backend/internal/handlers/lookup_test.go` — table-driven: empty arrays, single-per-type, mixed, partial miss, oversized batch (>256), missing auth, malformed body (decodeJSONStrict).

### 1.C UI design tokens (globals.css)

- [x] OCR-S1-05 Rewrite `ui/src/app/globals.css`: define `[data-theme="dark"]` Obsidian Clarity tokens (`--surface-container-{lowest,low,medium,high,highest}`, primary/secondary/tertiary + on-* variants, outline/outline-variant, error/error-container) and a coherent `[data-theme="light"]` counterpart (desaturated indigo on warm-white). Extend `@theme` block to map Tailwind colors to the new vars (`bg-surface-container-high`, `text-on-surface-variant`, etc.). Add `--font-display` / `--font-sans` mappings. Add utilities: `bg-blob-hero`, rewrite `glass-card` with `color-mix` translucent surface + 28px `backdrop-filter: blur(...)`, new `pulse-dot` keyframed dot. Keep existing `:focus-visible` and `fadeInUp`.

### 1.D Fonts + layout + Providers

- [x] OCR-S1-06 `bun add @tanstack/react-query @tanstack/react-query-devtools` in `ui/`.
- [x] OCR-S1-07 New `ui/src/components/providers.tsx` (client) — wraps `<QueryClientProvider>` + `<HydrationBoundary>`; absorbs Sonner `<Toaster richColors closeButton />` and `<ErrorBoundary>` mounts.
- [x] OCR-S1-08 Modify `ui/src/app/layout.tsx`: load `Fraunces` and `Inter` via `next/font/google` with `display: 'swap'`, expose CSS variables, attach to `<html>` className. Add `<div className="bg-blob-hero" aria-hidden />` inside `<body>` before `<Providers>`. Wrap children with `<Providers>`. Move existing Sonner Toaster + ErrorBoundary into `<Providers>`.

### 1.E TanStack Query infrastructure

- [x] OCR-S1-09 New `ui/src/lib/query-client.ts` — `makeQueryClient()` (`staleTime: 30_000`, `gcTime: 5*60_000`, `retry: 1`, `refetchOnWindowFocus: false`); `getQueryClient()` per-request via `cache()` for RSC.
- [x] OCR-S1-10 New `ui/src/lib/api-client.ts` — `request<T>(path, init)` wrapping `/api/proxy/...`, throwing typed `ApiError` on non-2xx.
- [x] OCR-S1-11 New `ui/src/lib/lookup-types.ts` — `LookupRequest`, `LookupResponse`, `ResolvedUser`, `ResolvedProject`, `ResolvedRole`, `ResolvedBundle`.
- [x] OCR-S1-12 New `ui/src/lib/queries/` directory with `useProjects.ts`, `useBundles.ts`, `useMappingRules.ts`, `useNameResolver.tsx` shipped this stage (proof-of-life consumes them). Remaining hooks (`useUsers`, `useApplications`, `useRoles`, `useAudit`, `useRequests`, `useGovernance`, `useTopology`, `useOperations`, `useGrants`) authored at the start of Stages 2-4 as their pages migrate.

### 1.F Name resolver + Name components

- [x] OCR-S1-13 New `ui/src/lib/queries/useNameResolver.ts` — context provider with tick-batched queues (rAF flush), single `useQuery(['lookup', sortedKey], …)` per batch, `staleTime: 5*60_000`. API: `useNameResolver()` → `{resolveUser, resolveProject, resolveRole, resolveBundle, prefetch(batch)}`. Mounted inside `<Providers>`.
- [x] OCR-S1-14 New `ui/src/components/names/{UserName,ProjectName,RoleName,BundleName,ResourceName,index}.tsx`. Each renders `<Skeleton/>` while loading. `?debug=ids` query flag reveals UID via tooltip. `<ResourceName kind/>` switches between User/Project/Role/Bundle based on `kind` prop.
- [x] OCR-S1-15 New `ui/src/lib/queries/__tests__/useNameResolver.test.tsx` — assert 50 mounts in one tick → exactly one network call; cache hit → no request; partial miss → loading resolves with name fallback.

### 1.G Base UI components

- [x] OCR-S1-16 New `ui/src/components/ui/Modal.tsx` — generic primitive: focus trap (port from current ConfirmModal internals), `aria-modal="true"`, Esc + click-outside dismiss, glass-card body. `ui/src/components/ui/__tests__/Modal.test.tsx` — focus trap, Esc, click-outside, aria-modal — authored **before** the ConfirmModal refactor.
- [x] OCR-S1-17 Refactor `ui/src/components/ui/ConfirmModal.tsx` to compose `<Modal>` + destructive footer. Preserve existing prop signature. Add `ui/src/components/ui/__tests__/ConfirmModal.test.tsx` to lock the a11y contract.
- [x] OCR-S1-18 New `ui/src/components/ui/Drawer.tsx` — right-side sheet variant of Modal.
- [x] OCR-S1-19 Rewrite `ui/src/components/ui/Card.tsx` with variants `default | glass | container`. Preserve existing `CardHeader` / `CardTitle` exports.
- [x] OCR-S1-20 Rewrite `ui/src/components/ui/Button.tsx` — pill default; variants primary (gradient `#818cf8 → #c084fc`, on-primary text, inset 1px white-10% stroke, ambient shadow) | secondary (surface-container-high) | ghost | outline | destructive | link; sizes sm/md/lg. Preserve current prop signature.
- [x] OCR-S1-21 Rewrite `ui/src/components/ui/Badge.tsx` — keep 8 existing variants, add `pulse?: boolean` overlay.
- [x] OCR-S1-22 New `ui/src/components/ui/Input.tsx` — pill, inner-shadow, focus ring.
- [x] OCR-S1-23 New `ui/src/components/ui/Select.tsx` — native `<select>` with pill styling.
- [x] OCR-S1-24 New `ui/src/components/ui/Pulse.tsx` — animated dot, variants success/warn/error/info.
- [x] OCR-S1-25 New `ui/src/components/ui/Eyebrow.tsx` — uppercase 12px label-cap, tracking 0.1em.
- [x] OCR-S1-26 Rewrite `ui/src/components/ui/EmptyState.tsx` — glass variant, eyebrow + headline + body + optional CTA.

### 1.H Proof-of-life page migration

- [x] OCR-S1-27 Migrate `ui/src/app/projects/page.tsx` to consume `useProjects()` hook (replaces manual `fetch("/api/proxy/projects")`). Other pages remain on legacy fetch until Stage 2-3.

### 1.I Verification + ADRs + detect_changes

- [x] OCR-S1-28 `cd backend && go vet ./... && go test ./...` — must be clean.
- [x] OCR-S1-29 `cd ui && bun run lint && bun run test && bun run build` — must be clean.
- [x] OCR-S1-30 `mcp__codebase-memory-mcp__detect_changes` to refresh graph.
- [x] OCR-S1-31 ADRs filed via `manage_adr` under a new `## Frontend Architecture` section: "Adopt TanStack Query as canonical client data layer", "Batch UID→name resolution via POST /api/v1/lookup", "Obsidian Clarity design tokens (dark-first, light counterpart)".
- [x] OCR-S1-32 `openspec validate obsidian-clarity-redesign --strict` ⇒ `Change 'obsidian-clarity-redesign' is valid`.

## Stage 2 — High-pain pages

### 2.A Dashboard (`/`)

- [x] OCR-S2-01 Cap stat grid at `xl:grid-cols-4`; glass-card stat cards over `bg-blob-hero`. AdminDashboard reduces hero from 5 → 4 high-signal cards (Pending Requests, Expiring Grants, Projects, Bundles) with `font-display` 5xl numerals and warn tone when counts > 0.
- [x] OCR-S2-02 Recent-activity row converted to xl:grid-cols-3 with the activity feed spanning 2 columns and a "Live operations pulse" rail in the third (top 3 in-flight intents via `useIntents({status:"in_flight", limit:3})`, polling every 5s).
- [x] OCR-S2-03 Activity feed actor and target render via `<UserName/>` (system events get a "System" label). Note: backend `AuditLog` has no `target_kind` field — Stage 2 routes targets through `<UserName/>` since `audit_logs.target_zitadel_user_id` is the canonical schema. ResourceName kind dispatch is reserved for future audit shape changes.
- [x] OCR-S2-04 New `useDashboardSummary()` in `ui/src/lib/queries/useDashboard.ts` composing `useGovernanceSummary` + `useAuditEntries({limit:20})` + `useProjects` + `useBundles`. Manual server-side fetch removed; the admin path delegates to the `<AdminDashboard>` client island.
- [x] OCR-S2-05 New `ui/src/app/__tests__/page.test.tsx` covers admin hero render, stat-grid `xl:grid-cols-4` cap, and `UUID_REGEX` regression assertion that no raw UUID escapes the rendered DOM after lookup resolution.

### 2.B Users (`/users`)

- [x] OCR-S2-06 Replaced 0.95fr/1.25fr split with `xl:grid-cols-[280px_1fr_1.4fr]` 3-column shell. Filter rail and lineage panel are sticky on xl. Virtualization deferred (the live directory is 5–50 users in production; native `max-h + overflow-y-auto` is sufficient — revisit if a deployment crosses 500).
- [x] OCR-S2-07 Filter rail ships search-by-text + project-name filter pills derived from `key_projects`. Pills toggle multi-select; "Clear filters" link resets state. UUID dropdowns are gone.
- [x] OCR-S2-08 Lineage tree uses 1px `border-l-2 border-primary-container` (source) and `border-[var(--success)]` (derived) guides; column headers use `<Eyebrow tone="primary">` / `<Eyebrow tone="muted">`. Project names → `<ProjectName/>`, role labels → `<RoleName/>`, "Granted by" line on direct grants → `<UserName/>`.
- [x] OCR-S2-09 `useUsers(q)`, `useUserAccess(id)`, `useUserGrants(id)`, `useAssignBundle(id)`, `useCreateGrant(id)` authored in `ui/src/lib/queries/useUsers.ts`. All manual `fetch("/api/proxy/...")` call sites removed from `users/page.tsx`.
- [x] OCR-S2-10 `<ConfirmModal/>` retained for the bundle-assign flow (the only mutation gated on confirmation). Composed atop the new `<Modal/>` per OCR-S1-17 — a11y contract validated by the Stage 1 ConfirmModal test.
- [x] OCR-S2-11 New `ui/src/app/users/__tests__/page.test.tsx` — project pill name rendering + toggle, source/derived columns visible, ConfirmModal opens on Assign click, no-raw-UUID regex assertion after lookup resolution.

### 2.C Audit (`/audit`)

- [x] OCR-S2-12 Summary cards adopt `Card variant="glass"` with `<Eyebrow/>` labels and `font-display` 5xl numerals.
- [x] OCR-S2-13 Audit timeline switched from a `<table>` to a list of two-line clickable rows: top line `<Pulse/>` + action + `<UserName/>` actor → `<UserName/>` target; bottom line timestamp + resource_id (truncated, full UID surfaced via `title=`). Click opens a `<Drawer size="lg"/>` rendering the full entry via `<JsonView/>` with actor/target also resolved through `<UserName showEmail/>`.
- [x] OCR-S2-14 Watchlist row format `<UserName/> → <ProjectName/> : <RoleName/>` with a `<Pulse/>` whose variant scales with `describeExpiry` tone (info/warn/error). Cleanup hints rendered as glass-card list.
- [x] OCR-S2-15 All UID renders at the formerly-flagged lines now route through Name components. Actor filter `<Select>` keeps UUID-typed values for backend filter compatibility but reads the resolver cache so `<option>` text shows resolved display names; falls back to a truncated UID prefix while resolution is pending.
- [x] OCR-S2-16 `useAuditEntries({limit})` and `useGovernanceSummary()` from new hooks; `useWatchlist()` is a thin slice over the governance summary cache. "Load more" bumps the limit (20 → 200 cap) — true cursor pagination is a backend-side change deferred (handler ships only `?limit=`); the React Query key includes the limit so each step caches independently.
- [x] OCR-S2-17 New `ui/src/app/audit/__tests__/page.test.tsx` — actor select option labels resolve to display names (no UUIDs), watchlist row contains resolved user/project/role names, "Load more" triggers a second `/audit` fetch, and no UUIDs leak into the rendered timeline.

### 2.D Stage 2 verification

- [x] OCR-S2-18 `bun run lint && bun run test && bun run build` clean (51/51 tests across 8 files); `go test ./...` clean (280/280) and `go vet ./...` clean.
- [ ] OCR-S2-19 Manual checklist on each of `/`, `/users`, `/audit`: empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast. (Pending — requires the user to drive the dev server in a browser.)
- [x] OCR-S2-20 Spec deltas updated (see `specs/operational-readiness`, `specs/user-management`, `specs/access-governance`); `detect_changes` invocation pending the codebase-memory MCP heartbeat.

## Stage 3 — Remaining pages

- [x] OCR-S3-01 `ui/src/app/projects/page.tsx`: responsive `lg:grid-cols-2 2xl:grid-cols-3` glass-card layout with `<Eyebrow tone="primary">` + Fraunces (`font-display`) headline + the existing 4-stat ladder per project. The `useProjects` proof-of-life hook from Stage 1 already drives the page; lazy `useProjectTopology(id)` and Owner/created_by surfacing are deferred — `models.ProjectSummary` does not currently expose owner/created_by fields, and per-project topology already rolls up via the `roleIndex` derived from `useBundles + useBundleRolesByBundle + useMappingRules`. ConfirmModal-on-archive is out of scope (no backend route).
- [x] OCR-S3-02 `ui/src/app/applications/page.tsx`: rewritten on `useApplications` + `useTokenSimulator`. Fixed the height mismatch by wrapping the right column in `min-h-0 h-full flex flex-col` and giving the Simulator card `min-h-0`. Token Simulator now uses a `bg-surface-container-lowest` code-block surface with `<JsonView compareWith/>` for inline diff in compare mode. `<CopyButton/>` retained on every panel per OpenSpec. User IDs were not exposed on this page — persona names ride directly off the `/catalog` payload, so no `<UserName/>` swap was needed.
- [x] OCR-S3-03 `ui/src/app/bundles/page.tsx`: glass-card expandable rows. Role chips now resolve through `<ProjectName/>` + `<RoleName/>` instead of a `project?.name ?? id` fallback. The impact accordion is collapsed by default and only triggers `useBundleImpact(id)` when the operator opens it (verified by the dedicated bundles page test that asserts no `/bundles/{id}/impact` call fires until the toggle is clicked). Affected user list shows the first 5 via `<UserName/>` plus "+N more". `useCreateBundle` + `useAddBundleRole` mutations replaced manual fetches; Sonner toasts fire on success/error. ConfirmModal-on-delete deferred — backend has no `DELETE /bundles/{id}`; that route belongs to Stage 4.
- [x] OCR-S3-04 `ui/src/app/policies/page.tsx`: glass-card per rule with `<Eyebrow tone="primary">Mapping rule</Eyebrow>`, `<Pulse variant="info"/>` for the active state, and `<ProjectName/>` + `<RoleName/>` on every source/target chip. `CreateRuleForm` was rewritten as a controlled component that renders inside `<Modal/>` (focus-trapped, Esc + click-outside dismiss, validates the rule via `useValidateMappingRule` with debounced cycle warnings before submit). `useCreateMappingRule` + `useBumpMappingRule` mutations replaced manual fetches. Delete mutation deferred — backend has no `DELETE /rules/mapping/{id}`.
- [x] OCR-S3-05 `ui/src/app/requests/page.tsx` + `components/requests/{AdminRequestsView,UserRequestsView}.tsx`: admin queue is now a Linear-style list with `<Pulse/>` on every row (warn variant + a "Pending >24h" badge once the request is older than the 24h SLA). Member view uses the same `<Pulse/>` motif scaled down to a single status timeline. `useRequestsAdmin({status})` / `useRequestsMine()` / `useCreateRequest` / `useDecideRequest` replaced all `fetch("/api/proxy/requests")` calls. The previously-flagged `{requester_id} → {project_id}:{role_key}` UID render at `AdminRequestsView.tsx:245` is now `<UserName/> → <ProjectName/> · <RoleName/>` everywhere. ConfirmModal gates both Approve and Reject (the destructive variant is preserved for Reject).
- [x] OCR-S3-06 `ui/src/app/graph/page.tsx`: pan/zoom mechanics preserved verbatim per the topology-graph capability assertion. The Inspector moved into `<Drawer size="lg"/>` so node detail no longer steals canvas real estate. Legend pills now ride a floating top-left glass chip; zoom controls + scroll-hint use the same glass treatment in the top-right and bottom-right. `useTopology()` replaces the page-level `useEffect+fetch`; `<ProjectName/>` resolves the per-node project badge inline.
- [x] OCR-S3-07 `ui/src/app/zitadel/page.tsx`: new `<LiveStatusTile/>` glass card at the top of the page polls `useZitadelHealth()` every 10s and surfaces a `<Pulse/>` whose variant flips between steady-success (`ok` + `static`), amber-pulse (`disabled`), and red-pulse (`error` / unreachable). System-mode awareness already ships in the Sidebar (`SystemModeBadge`) so the page header reuses that chrome rather than duplicating it. The deeper CRUD sections (Projects, Users, Grants, Rotation) keep their imperative `apiGet`/`apiSend` shape — full hook migration of those is out of stage scope.
- [x] OCR-S3-08 `ui/src/app/login/page.tsx`: hero treatment uses Fraunces (`font-display`) on the 5–6xl headline, three eyebrow-tagged glass tiles for the role split, and a `<Card variant="glass"/>` for the login form. The global `bg-blob-hero` mounted by `app/layout.tsx` provides the soft gradient background. Demo-cookie rejection in OIDC mode is unchanged — `process.env.ZITADEL_DOMAIN` still gates the `<DemoIdentityCard/>` at both render branches.
- [x] OCR-S3-09 Stage 3 verification: `bun run lint && test && build` clean (62/62 tests across 11 files); `cd backend && go vet ./... && go test ./...` clean (280/280). Per-page tests added for `/requests`, `/bundles`, `/policies` asserting the no-raw-UUID regression contract plus the impact-accordion lazy-fetch invariant. Spec deltas updated under `specs/operational-readiness`, `specs/topology-graph`, `specs/access-governance`, `specs/application-claims`. `detect_changes` invoked. Manual per-page browser checklist tracked at OCR-S3-10.
- [ ] OCR-S3-10 Manual per-page checklist on each Stage 3 route (`/projects`, `/applications`, `/bundles`, `/policies`, `/requests`, `/graph`, `/zitadel`, `/login`): empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast. Pending — requires the user to drive the dev server in a browser.

## Stage 4 — Orphan surfacing

### 4.A Bundle CRUD

- [x] OCR-S4-01 New `ui/src/components/bundles/CreateBundleModal.tsx` — Modal-wrapped name+description form; `useCreateBundle()` mutation invalidates the bundle list cache; Sonner toast on success. Mounted from the `/bundles` toolbar (replaces the Stage 3 inline form). Deferred: explicit project-scope and initial-roles picker — bundles are global containers in this data model and roles attach via `<AddRolesToBundlePicker/>` after creation, keeping the create flow minimal.
- [x] OCR-S4-02 New `ui/src/components/bundles/AddRolesToBundlePicker.tsx` — searchable role list grouped by project, multi-select chips, batched sequential `useAddBundleRole()` calls (backend accepts one role per POST). Roles already in the bundle render as disabled "Already in bundle". A failure mid-batch stops the loop and keeps un-added selections in the picker for retry.
- [x] OCR-S4-03 New `ui/src/components/bundles/BundleImpactAccordion.tsx` — extracted self-contained collapsible. Defers `useBundleImpact(id)` until opened (verified by the bundles page test). Renders the first 10 affected users via `<UserName/>` + a "+N more" overflow chip.

### 4.B Role authoring

- [x] OCR-S4-04 New `ui/src/components/roles/CreateRoleModal.tsx` — fields project (Select), display_name, role_key (slug-derived from display name with one-way `keyTouched` lock so manual edits stick), description, clone_from (project-scoped Select). `useCreateRole()` mutation. 409 CONFLICT surfaces inline as a field-level error on role_key without closing the modal; other failures fall through to a Sonner toast. Mounted from the `/bundles` toolbar via `+ Create role`. Claims editor deferred — the backend accepts the canonical fields (project_id/role_key/display_name/description/group/clone_from) and Stage 4 wires those.

### 4.C Operations queues page

- [x] OCR-S4-05 New `ui/src/app/operations/page.tsx` (admin-only RSC + `OperationsClient` island). Three tabs (Intents / Webhook events / Onboarding triggers); each polls every 5s via the per-resource hook and pauses while the tab is hidden. Status filter pills are exposed only on the Intents tab (the underlying `?status=` query parameter is the only one universally honored). Each row shows a `<Pulse/>` for status, resolved user/project names, relative age, truncated last error with full text on hover, and a "Payload" button that opens the row in a `<Modal/>` + `<JsonView/>`.
- [x] OCR-S4-06 Modify `ui/src/components/SidebarNav.tsx` — added "Admin" eyebrow section in the admin sidebar branch with "Operations" → `/operations` and "Grants" → `/grants`. The existing "Operations" section keeps `/zitadel` for diagnostic continuity.

### 4.D Global grants + reconciliation

- [x] OCR-S4-07 New `backend/internal/handlers/reconciliation.go` — `GET /api/v1/reconciliation/grants` wired under `withOperatorAuth` in `router.go`. Returns `{only_in_mkauth, only_in_zitadel, drift, generated_at, truncated}`. Reuses `services.AllDirectGrants` (new wrapper around `db.GetAllDirectGrants` that filters expired rows) and the existing `zitadelListAllGrants` injectable. Pure comparison core (`computeReconciliationDiff`) extracted for test isolation. Truncation flag set when Zitadel reports more grants than the 1000-row page returned.
- [x] OCR-S4-08 New `backend/internal/handlers/reconciliation_test.go` — eight cases covering only-in-mkauth, only-in-zitadel, role-mismatch, role-superset, aligned (no drift emitted), truncation flag propagation, Zitadel failure surfaces 502, and stable multi-pair ordering. All pass.
- [x] OCR-S4-09 New `ui/src/app/grants/page.tsx` (admin-only RSC + `GrantsClient` island). Two tabs: "All grants" (unioned ledger from `useZitadelAllGrants` + `useReconciliationDiff` + `useMappingRules` for source attribution) and "Reconciliation" (drift snapshot with three count cards + categorized lists). Each All-grants row carries a Source pill ("MkAuth + Zitadel" / "Zitadel only" / "Derived from rule" / "MkAuth only (sync gap)"). Drift rows open a side-by-side `<Drawer/>` rendering both records via `<JsonView/>`. Strictly read-only — no Apply/Sync buttons.
- [x] OCR-S4-10 Sidebar Grants link added in OCR-S4-06.

### 4.E Stage 4 verification

- [x] OCR-S4-11 `go vet ./... && go test ./...` clean (288/288 — was 280, +8 reconciliation tests).
- [x] OCR-S4-12 `bun run lint && bun run test && bun run build` clean (73/73 across 16 files; new routes ship at `/operations` 4.97kB + `/grants` 8.24kB).
- [x] OCR-S4-13 Manual: `/operations` and `/grants` walked in browser. Confirmed: admin-gate redirect for non-admins on both routes; sidebar Admin section visible only to admins; reconciliation Drawer a11y (Tab cycle, Esc, focus restore, side-by-side `JsonView`); polling pause on hidden tab. Operations Intents tab observed empty against the demo backend — distinct architectural finding (`provisioning_intents` is only written from the Zitadel webhook + expiry scheduler paths, not from MkAuth-UI direct grants), not a Stage 4 regression. Payload Modal a11y + open-on-row contract is covered by `Modal.test.tsx` (focus trap / Esc / aria-modal) and `app/operations/__tests__/page.test.tsx` (row click → Modal opens with payload). Live Modal validation deferred until the demo Zitadel Actions v2 webhook is registered (separate from this redesign plan).
- [x] OCR-S4-14 Spec deltas finalized in `specs/operational-readiness/spec.md` (Bundle/role authoring Modal, Operations page, Grants page requirements added). `openspec validate obsidian-clarity-redesign --strict` ⇒ valid. ADR refresh + `detect_changes` invocation tracked alongside this stage.
- [ ] OCR-S4-15 Once all stages merged: `openspec/changes/obsidian-clarity-redesign` ready for `/opsx:archive` consideration.
