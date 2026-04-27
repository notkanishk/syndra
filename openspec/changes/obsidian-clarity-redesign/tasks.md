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
- [ ] OCR-S1-31 File ADRs via `manage_adr`: "Adopt TanStack Query as canonical client data layer", "Batch UID→name resolution via POST /api/v1/lookup", "Obsidian Clarity design tokens (dark-first, light counterpart)".
- [ ] OCR-S1-32 `openspec validate obsidian-clarity-redesign --strict` (if available).

## Stage 2 — High-pain pages

### 2.A Dashboard (`/`)

- [ ] OCR-S2-01 Cap stat grid at `xl:grid-cols-4`; grow each stat card with trend delta. Wrap in glass-card over `bg-blob-hero`.
- [ ] OCR-S2-02 Convert recent-activity row to 2/3 + 1/3 split. Right rail = "Live operations pulse" (top 3 in-flight intents from `useIntents`, admin-only).
- [ ] OCR-S2-03 Replace `page.tsx:239` `Target: ${entry.target_id}` with `<ResourceName kind={entry.target_kind} id=…/>`. Activity actors → `<UserName/>`.
- [ ] OCR-S2-04 New `useDashboardSummary()` composing `useGovernanceSummary` + `useAuditEntries({limit:20})`. Replace manual fetch.
- [ ] OCR-S2-05 New `ui/src/app/__tests__/page.test.tsx` — admin render, stat grid cap, no raw UUID regex assertion.

### 2.B Users (`/users`)

- [ ] OCR-S2-06 Replace 0.95fr/1.25fr split with 3-column shell: 280px filter rail / virtualized user list / sticky lineage panel.
- [ ] OCR-S2-07 Filter rail: search + role/project filter pills (combobox-style) replacing UUID dropdowns. Backed by `useUsers()` + `useProjects()`.
- [ ] OCR-S2-08 Lineage tree: 1px outline-variant guides; source vs derived via `<Eyebrow>SOURCE</Eyebrow>` / `<Eyebrow>DERIVED</Eyebrow>`. All UID renders → Name components. "Granted by" → `<UserName/>`.
- [ ] OCR-S2-09 Switch to `useUsers(filter)`, `useUserAccess(id)`, `useUserGrants(id)` hooks. Remove all manual fetch from this page.
- [ ] OCR-S2-10 Verify revoke flow continues to use `<ConfirmModal/>` (now via new `<Modal/>`).
- [ ] OCR-S2-11 New `ui/src/app/users/__tests__/page.test.tsx` — filter pills render names, two-source-path lineage renders, revoke opens ConfirmModal, no-raw-UUID assertion.

### 2.C Audit (`/audit`)

- [ ] OCR-S2-12 Summary cards adopt glass-card.
- [ ] OCR-S2-13 Two-line row layout (top: action + actor + target as resolved names; bottom: timestamp + resource + ID hover). Click opens `<Drawer/>` with full payload via `<JsonView/>`.
- [ ] OCR-S2-14 Watchlist becomes glass-card list; active escalations get `<Pulse/>`.
- [ ] OCR-S2-15 Replace UID renders at `audit/page.tsx:242-244, 96, 98, 310` with Name components / ResourceName. Actor filter values stay UUID-typed but render `<UserName/>` labels via custom `<Select/>`.
- [ ] OCR-S2-16 Switch to `useAuditEntries(filter)` (cursor pagination), `useWatchlist()`, `useGovernanceSummary()`. Remove manual fetch.
- [ ] OCR-S2-17 New `ui/src/app/audit/__tests__/page.test.tsx` — actor filter renders names, watchlist row format, cursor pagination, no-raw-UUID assertion.

### 2.D Stage 2 verification

- [ ] OCR-S2-18 `bun run lint && bun run test && bun run build` clean; `go test ./...` clean (backend untouched but verify).
- [ ] OCR-S2-19 Manual checklist on each of `/`, `/users`, `/audit`: empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast.
- [ ] OCR-S2-20 `detect_changes`; spec deltas updated.

## Stage 3 — Remaining pages

- [ ] OCR-S3-01 `ui/src/app/projects/page.tsx`: responsive 1-3 col glass cards, eyebrow + headline + 4 stats, lazy `useProjectTopology(id)` on expand. Owner/created_by → `<UserName/>`. ConfirmModal on archive.
- [ ] OCR-S3-02 `ui/src/app/applications/page.tsx`: fix height mismatch with `min-h-0 h-full` flex parent. Token Simulator gets glass-card with code-block surface + inline diff. `useApplications`, `useApplication`, `useTokenSimulator`. User IDs → `<UserName/>`. CopyButton retained per OpenSpec.
- [ ] OCR-S3-03 `ui/src/app/bundles/page.tsx`: glass-card expandable rows with smooth height; impact accordion (Stage 4 wiring) collapsed. `useBundles`, `useBundle`, `useBundleImpact`. Role chips → `<RoleName/>`; assignment first-5 `<UserName/>` + "+N". ConfirmModal on delete; Sonner on assign/unassign.
- [ ] OCR-S3-04 `ui/src/app/policies/page.tsx`: glass-card per rule with eyebrow ("MAPPING RULE"), pulse for active. CreateRuleForm in Modal. `useMappingRules` + create/delete mutations. Targets → `<RoleName/>`/`<ProjectName/>`. ConfirmModal on delete.
- [ ] OCR-S3-05 `ui/src/app/requests/page.tsx` + `components/requests/{AdminRequestsView,UserRequestsView}.tsx`: admin queue Linear-style with `<Pulse/>` on items >24h pending; member view with status timeline. `useRequestsAdmin`/`useRequestsMine` + mutations. `AdminRequestsView.tsx:245` UID render → Name components. ConfirmModal on approve/deny.
- [ ] OCR-S3-06 `ui/src/app/graph/page.tsx`: keep current pan/zoom (mandatory); node detail panel becomes glass `<Drawer/>`; legend pills → floating top-right glass chip. `useTopology`. Project IDs → `<ProjectName/>`.
- [ ] OCR-S3-07 `ui/src/app/zitadel/page.tsx`: glass-card tiles; status `<Pulse/>` (steady-green / amber-pulse / red-pulse); system mode badge integrated at header. `useZitadelDiagnostics()` with `refetchInterval: 10_000`.
- [ ] OCR-S3-08 `ui/src/app/login/page.tsx`: hero treatment with Fraunces, glass-card login form, blob background. No functional change. Demo cookie rejection unchanged.
- [ ] OCR-S3-09 Stage 3 verification: lint/test/build/`go test`; manual per-page checklist; `detect_changes`.

## Stage 4 — Orphan surfacing

### 4.A Bundle CRUD

- [ ] OCR-S4-01 New `ui/src/components/bundles/CreateBundleModal.tsx` — form with name, description, project scope (`<ProjectName/>` combobox), initial roles picker. `useCreateBundle()`. Mounted from `/bundles` toolbar.
- [ ] OCR-S4-02 New `ui/src/components/bundles/AddRolesToBundlePicker.tsx` — searchable role list grouped by project, multi-select chips, `useAddRoleToBundle()` (sequential or batch).
- [ ] OCR-S4-03 New `ui/src/components/bundles/BundleImpactAccordion.tsx` — collapsible inside bundle detail; `useBundleImpact(id)`; affected user count, sample list, role count delta.

### 4.B Role authoring

- [ ] OCR-S4-04 New `ui/src/components/roles/CreateRoleModal.tsx` — fields display_name, role_key (slug-derived w/ override), description, clone_from (Select), claims (advanced). `useCreateRole()` mutation; debounced uniqueness check or 409 → toast.

### 4.C Operations queues page

- [ ] OCR-S4-05 New `ui/src/app/operations/page.tsx` — admin-only RSC; non-admins redirect to `/`. Three tabs (Intents / Webhook events / Onboarding triggers) with status filter pills, `<Pulse/>` per row, `refetchInterval: 5_000`. "View payload" → `<Modal/>` with `<JsonView/>`.
- [ ] OCR-S4-06 Modify `ui/src/components/SidebarNav.tsx` — add "Admin" eyebrow section gated on session admin role; "Operations" link → `/operations`.

### 4.D Global grants + reconciliation

- [ ] OCR-S4-07 New `backend/internal/handlers/reconciliation.go` — `GET /api/v1/reconciliation/grants` returning `{only_in_mkauth: [...], only_in_zitadel: [...], drift: [...]}`. Reuse direct-grants repo + `ListUserGrants`. Wrap with `withOperatorAuth` (drift data is sensitive). Wire route in `router.go`.
- [ ] OCR-S4-08 New `backend/internal/handlers/reconciliation_test.go` — synthetic drift cases (only-mkauth / only-zitadel / role-mismatch / role-superset).
- [ ] OCR-S4-09 New `ui/src/app/grants/page.tsx` — admin-only. Tab 1 "All grants" via `useZitadelGrants(filter)`; Tab 2 "Reconciliation" via `useReconciliationDiff()`. Drift rows highlighted with amber outline + `<Pulse/>`; click → `<Drawer/>` with both records via `<JsonView/>`. Read-only.
- [ ] OCR-S4-10 Modify `ui/src/components/SidebarNav.tsx` — add "Grants" link → `/grants` under "Admin".

### 4.E Stage 4 verification

- [ ] OCR-S4-11 `cd backend && go vet ./... && go test ./...` — reconciliation_test.go passes.
- [ ] OCR-S4-12 `cd ui && bun run lint && bun run test && bun run build`.
- [ ] OCR-S4-13 Manual: `/operations` and `/grants` end-to-end on demo backend; admin gating returns non-admins to `/`; ConfirmModal a11y on every new flow.
- [ ] OCR-S4-14 `detect_changes`; spec deltas finalized; ADRs updated for backend reconciliation endpoint.
- [ ] OCR-S4-15 Once all stages merged: `openspec/changes/obsidian-clarity-redesign` ready for `/opsx:archive` consideration.
