## Why

`dashboard-ux-elevation` shipped the missing UX foundation (toast, ConfirmModal, theme toggle, audit grouping, topology pan/zoom) but the dashboard still has three structural problems that compound into poor day-to-day usability:

1. **Visual: generic and sparse.** The current tokens (vanilla indigo on neutral grays) read as default-Tailwind, not as a deliberate IAM console. The dashboard, applications, and graph pages have empty stretches at wide viewports; bundle and policies pages feel cramped. There is no cohesive design language to anchor the admin-console-first commitment in `syndra-core-architecture/design.md` § 5.
2. **Human readability: raw UIDs leak.** The audit log (`audit/page.tsx:242-244, 96, 98, 310`), request approval queue (`requests/AdminRequestsView.tsx:245`), dashboard activity feed (`page.tsx:239`), and graph node detail render Zitadel UUIDs verbatim. The actor filter on `/audit` is a `<select>` of UUIDs. None of this tells the operator *who* or *what* is involved — the central question of an admin console.
3. **Backend feature parity: orphaned endpoints.** The backend ships bundle creation (`POST /api/v1/bundles`), role authoring (`POST /api/v1/roles`), bundle impact preview (`GET /api/v1/bundles/{id}/impact`), provisioning intent visibility (`GET /api/v1/intents`), webhook event log (`GET /api/v1/webhook/events`), onboarding triggers (`GET /api/v1/onboarding/triggers`), and Zitadel global grants (`GET /api/v1/zitadel/grants`) — none of which have UI. Phase-5 OpenSpec calls for service-to-bundle visibility and reconciliation visibility, both gated by missing surfaces.

The data-fetching layer is also a root cause: every page uses `useState + useEffect + fetch("/api/proxy/...")` directly, producing duplicate requests, no cache, ad-hoc error handling, and no clean place to attach a UID→name resolver.

## What Changes

The change is staged across four tracks, executed in order. One umbrella OpenSpec change scopes all four — task IDs prefixed `OCR-S{n}-{nn}`.

### Track 1 — Foundation

- **Backend `POST /api/v1/lookup`**: new batch UID→name endpoint. Accepts `{user_ids[], project_ids[], role_keys[{project_id,role_key}], bundle_ids[]}`, returns a name map. Uses `decodeJSONStrict` and `withUserAuth`. Caps each array at 256. Tolerates partial misses. Reuses existing repository accessors — no new SQL.
- **Obsidian Clarity design tokens** in `ui/src/app/globals.css` (Tailwind v4 `@theme` block): full Material You-style palette (surface-container ladder, primary/secondary/tertiary, on-surface variants, outline/error). Dark theme is the new default aesthetic; a coherent light counterpart is authored alongside (no auto-inversion). New utilities: `bg-blob-hero`, `glass-card` (rewrite with `color-mix` translucency + 28px backdrop blur), `pulse-dot`.
- **Display font pairing**: Fraunces (variable serif) for h1/display surfaces via `next/font/google` with `display: 'swap'`; Inter remains the body face. CSS vars `--font-fraunces` and `--font-inter`.
- **TanStack Query** as canonical client data layer. New `<Providers>` mounts `QueryClientProvider` + `HydrationBoundary` at the root layout, absorbs the existing Sonner `<Toaster>` and `<ErrorBoundary>`. Per-resource hooks under `ui/src/lib/queries/` (one file per resource). New `ui/src/lib/api-client.ts` wraps `/api/proxy/...` with structured `ApiError`.
- **Name resolver**: `useNameResolver` context batches UIDs across one tick (rAF flush) into a single `/api/v1/lookup` query keyed by sorted concatenation. Companion components `<UserName/>`, `<ProjectName/>`, `<RoleName projectId roleKey/>`, `<BundleName/>`, `<ResourceName kind id/>` render skeletons until resolved. Optional `?debug=ids` flag reveals the raw UID via tooltip.
- **Base components** rewritten for Obsidian Clarity: `Card` (variants `default | glass | container`), `Button` (pill default, gradient primary, inset white-stroke + ambient shadow), `Badge` (adds `pulse?` overlay), `EmptyState` (glass variant). New: `Input` (pill, inner-shadow), `Select` (native pill), `Modal` (extracts focus-trap from current ConfirmModal), `Drawer` (right-side sheet variant), `Pulse` (animated status dot), `Eyebrow` (uppercase 12px label-cap). `ConfirmModal` is refactored to compose `Modal` + destructive footer — its a11y contract is captured in tests **before** the refactor.
- **Proof-of-life migration**: `/projects` is converted to React Query as a wiring smoke test. Other pages remain on legacy fetch until Tracks 2-3.

### Track 2 — High-pain pages (Dashboard, Users, Audit)

- **`/` dashboard**: stats grid capped at `xl:grid-cols-4` (was 5; ultra-wide viewports were sparse), each stat card grows to include trend delta. Hero stats wrap a `glass-card` over `bg-blob-hero`. Recent activity becomes 2/3 + 1/3 split with a "Live operations pulse" right rail (top 3 in-flight intents, admin-only). Activity actor + target render via `<UserName/>`/`<ResourceName/>`.
- **`/users`**: 3-column shell — left 280px filter rail (search + role/project filter pills replacing UUID dropdowns), middle virtualized list of glass user cards, right sticky lineage panel. Lineage tree uses 1px outline-variant guides; source vs derived distinguished by `<Eyebrow>SOURCE</Eyebrow>`/`<Eyebrow>DERIVED</Eyebrow>`. All UIDs resolved via Name components. Revoke flow continues to use ConfirmModal (now backed by new `Modal`).
- **`/audit`**: summary cards adopt glass-card. Two-line row layout — top: action + actor + target as resolved names; bottom: timestamp + resource + ID hover. Detail opens a `Drawer` showing full payload via `<JsonView/>`. Watchlist becomes glass-card list with `<Pulse/>` for active escalations. Actor filter values stay UUID-typed for backend compatibility but render `<UserName/>` labels via custom `<Select>`.

### Track 3 — Remaining pages

`/projects`, `/applications`, `/bundles`, `/policies`, `/requests`, `/graph`, `/zitadel`, `/login` — each gets the design-token reskin, switches to the new Query hooks, replaces UID renders with Name components, and preserves OpenSpec-mandated behaviors (Token Simulator copy/highlight/compare, Topology pan/zoom + deeplinks, ConfirmModal on every destructive flow, system mode badge null in steady-state).

### Track 4 — Orphan surfacing

- **Bundle CRUD**: `CreateBundleModal`, `AddRolesToBundlePicker`, `BundleImpactAccordion` mounted from `/bundles` toolbar. Wires existing `POST /bundles`, `POST /bundles/{id}/roles`, `GET /bundles/{id}/impact`.
- **Role authoring**: `CreateRoleModal` opened from project detail; supports `clone_from` prefill via existing `POST /roles`.
- **`/operations` (NEW route)**: admin-only. Three tabs — Intents / Webhook events / Onboarding triggers — surface `GET /intents`, `/webhook/events`, `/onboarding/triggers`. Status filter pills, `<Pulse/>` per row, `refetchInterval: 5000`. Payload viewer in a `Modal` with `<JsonView/>`.
- **`/grants` (NEW route)**: admin-only. Tab 1 "All grants" surfaces `GET /api/v1/zitadel/grants`. Tab 2 "Reconciliation" renders side-by-side Syndra-vs-Zitadel diff backed by NEW backend `GET /api/v1/reconciliation/grants` returning `{only_in_syndra[], only_in_zitadel[], drift[]}` — visibility only, no remediation.
- **Sidebar**: new "Admin" eyebrow section gated on session admin role; items "Operations" → `/operations`, "Grants" → `/grants`.

## Capabilities

### Modified Capabilities

- `operational-readiness`: Obsidian Clarity design tokens contract (dark + light counterpart), Pulse status indicator semantics, TanStack Query as canonical client data layer with per-resource hook directory, `POST /api/v1/lookup` endpoint contract, `GET /api/v1/reconciliation/grants` endpoint contract, `<Drawer>` primitive, `/operations` and `/grants` admin routes, `ConfirmModal` a11y contract preserved through the `Modal` refactor.
- `user-management`: every UID display surface MUST resolve via `<UserName/>`/`<ProjectName/>`/`<RoleName/>`/`<BundleName/>`; raw IDs allowed only behind a debug affordance. Filter rails replace UUID dropdowns with combobox pills.
- `access-governance`: audit table row format uses resolved names; watchlist row format `<UserName/> → <ProjectName/>:<RoleName/>`; audit detail opens a Drawer with full payload via JsonView.
- `application-claims`: NEW operator surface — global grants viewer (`/grants` tab 1) and reconciliation diff (`/grants` tab 2). Token Simulator copy/highlight/compare preserved through the Card variant migration.

### No spec change required

- `topology-graph`, `role-management`, `service-catalog`, `automation-policies`, `demo-catalog`: behavioral contracts unchanged; redesign is visual + data-layer plumbing only. Pan/zoom + node deeplinks (topology), per-role bundle/rule rollups (project view), member service catalog flow (service-catalog), live mapping-rule preview (automation-policies), and demo-catalog gating semantics all continue to hold.

## Impact

- **Backend additions** (small, scoped, additive only):
  - `POST /api/v1/lookup` (Stage 1)
  - `GET /api/v1/reconciliation/grants` (Stage 4)
- **Frontend additions** (Stage 1):
  - `ui/src/components/providers.tsx`
  - `ui/src/lib/query-client.ts`, `ui/src/lib/api-client.ts`, `ui/src/lib/lookup-types.ts`
  - `ui/src/lib/queries/{useUsers,useProjects,useApplications,useBundles,useRoles,useAudit,useRequests,useGovernance,useTopology,useMappingRules,useOperations,useGrants,useNameResolver}.ts`
  - `ui/src/components/names/{UserName,ProjectName,RoleName,BundleName,ResourceName,index}.tsx`
  - `ui/src/components/ui/{Input,Select,Modal,Drawer,Pulse,Eyebrow}.tsx`
  - Rewrites: `ui/src/components/ui/{Card,Button,Badge,EmptyState,ConfirmModal}.tsx`
- **Frontend additions** (Stage 4):
  - `ui/src/app/operations/page.tsx`, `ui/src/app/grants/page.tsx`
  - `ui/src/components/bundles/{CreateBundleModal,AddRolesToBundlePicker,BundleImpactAccordion}.tsx`
  - `ui/src/components/roles/CreateRoleModal.tsx`
- **Dependencies** (Stage 1):
  - `@tanstack/react-query` (~13 KB gz) and `@tanstack/react-query-devtools` (dev-only).
  - `next/font/google` exposes `Fraunces` (variable, ~30 KB woff2 subset).
- **Test coverage**:
  - Backend: `lookup_test.go` (auth/empty/partial/mixed/oversized/malformed), `reconciliation_test.go` (synthetic drift cases). Existing 272+ tests continue to pass.
  - UI: `useNameResolver.test.tsx` (batching: 50 mounts → 1 request, cache hit, partial miss), `Modal.test.tsx` (focus trap, Esc, click-outside, aria-modal — authored **before** ConfirmModal refactor), `ConfirmModal.test.tsx` (a11y unchanged), per-page "no raw UUID escapes the page" regex assertions on `/`, `/users`, `/audit`.
- **Migrations**: none required.
- **Breaking contract changes**: none. All API additions are net-new endpoints; existing endpoints retain their schemas. The frontend prop signatures of `Card`, `Button`, `Badge`, `EmptyState`, `ConfirmModal` are preserved so existing callers continue to work during migration.
- **Demo mode preserved**: every change uses catalog-driven defaults; nothing new branches on `ZITADEL_DOMAIN` in the UI. Demo cookies continue to be rejected in OIDC mode (`getSession()` check unchanged).
- **Light theme**: a coherent counterpart is authored deliberately, validated against WCAG AA on text and primary-on-primary-container button states. The toggle in the sidebar footer remains.
