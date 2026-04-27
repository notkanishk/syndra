# Design

## 1. Visual system: Obsidian Clarity

The aesthetic philosophy is "Sculptural Security" — uncompromising rigidity (security context demands it) balanced by an organic, fluid surface treatment. Depth is the primary navigator, achieved through translucent layering, backdrop blur, and ambient shadow rather than hard borders.

### 1.1 Token contract

Tokens are CSS variables defined under `[data-theme="dark"]` (default) and `[data-theme="light"]` (counterpart) in `ui/src/app/globals.css`. The Tailwind v4 `@theme` block maps Tailwind utility names (`bg-surface-container-high`, `text-on-surface-variant`, `border-outline`) to these variables so utilities work uniformly.

| Role | Dark | Light | Usage |
|---|---|---|---|
| `--background` / `--surface` | `#0c1324` | `#fbfaff` | Page canvas |
| `--surface-container-lowest` | `#070d1f` | `#ffffff` | Code blocks, recessed wells |
| `--surface-container-low` | `#151b2d` | `#f5f3ff` | Skeleton shimmer base |
| `--surface-container` | `#191f31` | `#efedfa` | Standard card surface |
| `--surface-container-high` | `#23293c` | `#e8e5f4` | Elevated card |
| `--surface-container-highest` | `#2e3447` | `#dfdcef` | Floating modal/drawer |
| `--on-surface` | `#dce1fb` | `#1a1d2e` | Primary body text |
| `--on-surface-variant` | `#c6c5d5` | `#4a4d63` | Muted/eyebrow text |
| `--primary` | `#bdc2ff` | `#5057f7` | CTA fill |
| `--on-primary` | `#131e8c` | `#ffffff` | Text on primary |
| `--primary-container` | `#818cf8` | `#dfe3ff` | Active state, focus ring |
| `--secondary` | `#ddb8ff` | `#7c3aed` | Secondary CTA |
| `--secondary-container` | `#62259b` | `#ede4ff` | Secondary surfaces |
| `--tertiary` | `#bec6e0` | `#666c80` | Decorative accent |
| `--outline` | `#908f9e` | `#7a7986` | Hard borders (rare) |
| `--outline-variant` | `#454653` | `#cbcad6` | Soft dividers, lineage guides |
| `--error` / `--on-error` | `#ffb4ab` / `#690005` | `#ba1a1a` / `#ffffff` | Destructive |
| `--error-container` | `#93000a` | `#ffdad6` | Error fill |

The light counterpart is **deliberate** — not auto-inverted. Every on-surface text and primary-on-primary-container button MUST validate WCAG AA (4.5:1 for body, 3:1 for large display). A vitest contrast assertion is feasible via `culori` or similar; the minimum bar is a Storybook + axe-core sweep.

### 1.2 Utilities

- `bg-blob-hero` — fixed pseudo-element, two large radial gradients (`#818cf8` @ 12% + `#c084fc` @ 10%), heavy blur, low z-index, `pointer-events:none`. Mounted once inside `<body>` of root layout. Provides atmospheric depth on hero areas.
- `glass-card` (rewrite) — `background: color-mix(in srgb, var(--surface-container) 70%, transparent)`, `backdrop-filter: blur(28px)`, `box-shadow: 0 30px 60px -20px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.06)`. Replaces the current solid-bg card.
- `pulse-dot` — keyframed opacity+scale (1.0 → 1.4 → 1.0; opacity 0.6 → 1.0 → 0.6); 1.6s infinite. Used by `<Pulse/>` and `<Badge pulse/>`.

### 1.3 Typography

- **Body**: Inter via `next/font/google`, weights 400/500/600; CSS var `--font-inter`.
- **Display**: Fraunces (variable serif) via `next/font/google`, axes wonk (default 0) and soft (default 0); CSS var `--font-fraunces`. Applied via `font-display` Tailwind utility on h1/h2 hero elements only. `display: 'swap'`, fallback adjusted against Inter to minimize layout shift.

| Level | Family | Size / Weight / Tracking / Line-height |
|---|---|---|
| display-lg | Fraunces | 48px / 600 / -0.02em / 1.1 |
| headline-md | Inter | 32px / 500 / -0.01em / 1.2 |
| title-sm | Inter | 20px / 500 / 0.01em / 1.4 |
| body-base | Inter | 16px / 400 / 0.01em / 1.6 |
| label-caps | Inter | 12px / 600 / 0.1em / 1.0 (uppercase via `<Eyebrow/>`) |

### 1.4 Shapes

Pills (rounded-full) for buttons, badges, inputs, selects. Cards use 24-32px radii (`rounded-card` = 1.5rem, `rounded-card-lg` = 2rem). The pill-vs-card divide is intentional: interactive elements feel touchable, container surfaces feel sculpted.

### 1.5 Pulse status semantics

| Variant | Color | Meaning |
|---|---|---|
| `success` | `#34d399` | Steady = healthy live |
| `warn` | `#fbbf24` | Pulsing = attention needed (warming, expiring) |
| `error` | `#f87171` | Pulsing fast = degraded/failed |
| `info` | `#60a5fa` | Pulsing slow = in-flight/processing |

Steady success uses the `Pulse` shape without animation; the others animate. `<Badge pulse>{children}</Badge>` overlays a small `pulse-dot` on the badge end.

## 2. Data layer: TanStack Query

### 2.1 ADR — Adopt TanStack Query

**Status**: Accepted (this change).

**Context**: Every page in `ui/src/app/` uses `useState + useEffect + fetch("/api/proxy/...")`. This produces:
- Duplicate requests (sidebar fetches `/governance/summary`; audit page fetches it again — no shared cache).
- No request dedupe across components mounting the same resource.
- Ad-hoc loading/error state per page.
- No clean integration point for batch UID resolution (the name resolver needs cache locality).
- No mutation invalidation pattern (post-create the page must refetch manually).

**Decision**: Adopt `@tanstack/react-query` v5 as the canonical client data layer. One `QueryClient` per request (RSC) via `cache(makeQueryClient)`; one shared `QueryClient` per browser session via `<QueryClientProvider>`. Per-resource hooks under `ui/src/lib/queries/` are the only way pages talk to the backend going forward.

**Alternatives considered**: SWR (smaller but lacks first-class mutations + invalidation), manual fetch with module-level cache (reinvents react-query at lower quality), pure RSC + server actions (requires structural rewrite of every page; defer until Stage 5+ if at all).

**Consequences**:
- +13 KB gz to client bundle (acceptable for an admin console).
- SSR hydration must be carefully managed — see § 2.3.
- The codebase gains a dedicated `queries/` directory; prop drilling of fetched data is replaced by hook calls.

### 2.2 Hook directory

Per-resource hooks under `ui/src/lib/queries/`. Each file exports list/detail queries plus mutations. Query keys follow the pattern `[resource, ...params]` so invalidation is precise.

| File | Hooks |
|---|---|
| `useUsers.ts` | `useUsers(params)`, `useUser(id)`, `useUserAccess(id)`, `useUserGrants(id)` |
| `useProjects.ts` | `useProjects()`, `useProject(id)` |
| `useApplications.ts` | `useApplications()`, `useApplication(id)`, `useTokenSimulator(appId,userId)` (mutation) |
| `useBundles.ts` | `useBundles()`, `useBundle(id)`, `useBundleImpact(id)`, `useCreateBundle()`, `useAddRoleToBundle()`, `useAssignBundle()` |
| `useRoles.ts` | `useRolesByProject(pid)`, `useCreateRole()` |
| `useAudit.ts` | `useAuditEntries(params)` (cursor-paginated via `useInfiniteQuery`) |
| `useRequests.ts` | `useRequestsAdmin()`, `useRequestsMine()`, `useApproveRequest()`, `useDenyRequest()` |
| `useGovernance.ts` | `useGovernanceSummary()`, `useWatchlist()` |
| `useTopology.ts` | `useTopology()` |
| `useMappingRules.ts` | `useMappingRules()`, `useCreateMappingRule()`, `useDeleteMappingRule()` |
| `useOperations.ts` | `useIntents`, `useWebhookEvents`, `useOnboardingTriggers` (`refetchInterval: 5_000`) |
| `useGrants.ts` | `useZitadelGrants(filter)`, `useReconciliationDiff()` |
| `useNameResolver.ts` | See § 3 |

### 2.3 SSR / RSC strategy

The current dashboard is mostly client-rendered with an RSC shell that reads `getSession()`. To prevent hydration mismatches:

1. **One QueryClient per request** in RSC: `getQueryClient()` uses `cache()` from `react`, so concurrent requests don't share mutable state.
2. **HydrationBoundary** wraps page-level RSC pre-fetches: where a page wants SSR-cached data (e.g. `/projects` fetched at the server), it `prefetchQuery`s on the QueryClient and dehydrates the state into the boundary; the client picks up where the server left off.
3. **Fully-client pages** (most current pages) skip the hydration boundary and just use the shared `<QueryClientProvider>`. Initial fetch happens on mount.
4. **`<Providers>` is a client component** (`'use client'`); the root layout RSC mounts it once around `{children}`.

## 3. Name resolution protocol

### 3.1 Endpoint contract — `POST /api/v1/lookup`

Auth: `withUserAuth` (the resolver returns metadata only, not authorization decisions).

Request:

```json
{
  "user_ids":    ["uid-1", "uid-2"],
  "project_ids": ["pid-1"],
  "role_keys":   [{"project_id": "pid-1", "role_key": "lab_member"}],
  "bundle_ids":  ["bid-1"]
}
```

All arrays optional; each capped at 256. Empty arrays valid (returns `{}` for that type).

Response (200):

```json
{
  "users":    {"uid-1": {"display_name": "Anita Sharma", "email": "anita@..."}},
  "projects": {"pid-1": {"name": "3D Lab"}},
  "roles":    {"pid-1:lab_member": {"display_name": "Member"}},
  "bundles":  {"bid-1": {"name": "Lab core"}}
}
```

Partial misses: missing IDs are simply absent from the map; never a 404. The response body always contains all four top-level keys (`users`/`projects`/`roles`/`bundles`) — empty objects when nothing matched.

Error cases:
- 400 `VALIDATION_FAILED` if any array exceeds 256 entries (`details: {field: "user_ids", reason: "max 256"}`).
- 400 if body fails `decodeJSONStrict` (unknown fields, multiple JSON values).
- 401 `UNAUTHORIZED` if no/invalid bearer token.

### 3.2 Implementation note

Reuse existing repository accessors used by `users.go`, `projects.go`, `bundles.go`, `roles.go` — no new SQL. Each entity-type lookup runs as an independent in-process call; partial misses on one type don't fail the others.

### 3.3 Client-side batching

`useNameResolver` is a context provider mounted inside `<Providers>`. It maintains four queues (`userIds`, `projectIds`, `roleKeys`, `bundleIds`) and a `pendingFlush` flag.

On `request*(id)`:
1. If `id` already in queue, no-op.
2. Else add to queue, schedule `requestAnimationFrame(flush)` if not already scheduled.

On flush:
1. Build sorted, deterministic `queryKey = ['lookup', JSON.stringify({u: sortedUserIds, p: sortedProjectIds, r: sortedRoleKeys, b: sortedBundleIds})]`.
2. Call `useQuery(queryKey, () => apiClient.post('/api/v1/lookup', batch))` with `staleTime: 5*60_000`.
3. Each `<UserName id=…/>` reads from the cache via the same queryKey + a selector. If the id isn't in the cached batch (rare race), it falls through to a follow-up resolver tick.

The vitest assertion: render 50 `<UserName/>` mounted in one `act()` cycle → exactly one `fetch` invocation against `/lookup`.

### 3.4 Display contract

Every Name component:
- Renders `<Skeleton className="w-16 h-4 inline-block"/>` while loading.
- Renders the resolved name on success.
- On miss/error, renders the `fallback` prop (default `—`) with the raw UID exposed via `title` attribute (so `cmd+click → inspect` reveals it without cluttering the visible page).
- When `?debug=ids` query flag present, hovers show a tooltip with the raw UID for any operator audit needs.

## 4. Component inventory delta

### 4.1 Rewritten

- `Card.tsx` — variants `default | glass | container`. `default` keeps current solid surface (used by skeletons + fallback paths); `glass` is the new primary for content cards; `container` is no-shadow surface-container fill. Existing `CardHeader`/`CardTitle` exports preserved.
- `Button.tsx` — pill default. Variants:
  - `primary`: gradient `linear-gradient(135deg, #818cf8, #c084fc)`, `var(--on-primary)` text, inset 1px white-10% top-stroke, ambient shadow `0 8px 16px -4px rgba(129,140,248,0.4)`.
  - `secondary`: `var(--surface-container-high)` fill, `var(--on-surface)` text.
  - `ghost`: transparent fill, hover `var(--surface-container-low)`.
  - `outline`: 1px `var(--outline-variant)` border, transparent fill.
  - `destructive`: `var(--error-container)` fill, `var(--on-error)` text.
  - `link`: text only, underline-on-hover.
  - Sizes `sm`/`md`/`lg`. Existing prop signature preserved (`isPending` spinner, etc.).
- `Badge.tsx` — keep 8 variants (default/secondary/outline/destructive/success/warning/info). New `pulse?: boolean` overlays `pulse-dot` at the badge end.
- `EmptyState.tsx` — glass variant: eyebrow + headline + body + optional CTA. Replaces dashed-border treatment.
- `ConfirmModal.tsx` — refactored to compose new `<Modal/>` + destructive footer. Tests captured before refactor (focus trap, Esc, click-outside, aria-modal).

### 4.2 New

- `Modal.tsx` — generic primitive: focus trap, `aria-modal="true"`, Esc + click-outside dismiss, glass-card body. Supports `size` prop (`sm`/`md`/`lg`) and `dismissible` (defaults true).
- `Drawer.tsx` — right-side sheet variant. Slides in from right; same focus + Esc + dismiss semantics.
- `Input.tsx` — pill `rounded-full`, inner-shadow `inset 0 1px 2px rgba(0,0,0,0.4)`, focus ring `var(--primary-container)`. `disabled` reduces opacity to 0.5.
- `Select.tsx` — native `<select>` with pill styling. (Custom multi-select wrapper deferred until Stage 4 needs it.)
- `Pulse.tsx` — animated dot, variants success/warn/error/info. `static?` prop renders without animation (steady-state).
- `Eyebrow.tsx` — uppercase 12px, `--font-inter`, weight 600, tracking 0.1em. Used as section labels.

### 4.3 Name components (new directory `components/names/`)

- `UserName.tsx` — `<UserName id="…" fallback="—" showEmail?={boolean}/>`.
- `ProjectName.tsx` — `<ProjectName id="…" fallback="—"/>`.
- `RoleName.tsx` — `<RoleName projectId="…" roleKey="…" fallback="…"/>`. Falls back to `roleKey` if unresolved.
- `BundleName.tsx` — `<BundleName id="…" fallback="—"/>`.
- `ResourceName.tsx` — switch on `kind` (user/project/role/bundle) dispatching to the right component. Used by audit table where `target_kind`/`resource_kind` is a runtime value.

## 5. Phasing rationale

Tokens + components first means the design migration is invisible to users initially (pages re-skin via the token swap) and creates one merge point per stage. The high-pain pages (Dashboard, Users, Audit) ship in Stage 2 because they have the worst UID exposure and density issues — fixing them first delivers the most operator value per unit of change. Stage 3 sweeps the rest. Stage 4's orphan surfacing is gated by Stage 1's TanStack Query infrastructure (the new pages all need it); shipping it last also ensures the design language is settled before adding new surfaces.

## 6. Risk register

1. **TanStack Query SSR hydration mismatches** — mitigated by `cache()`-based per-request QueryClient, `HydrationBoundary` per page that needs SSR data, full `'use client'` for everything else.
2. **ConfirmModal a11y regression during Modal refactor** — mitigated by capturing tests on Modal + ConfirmModal *before* the refactor lands; failure is a release blocker per OpenSpec.
3. **`/api/v1/lookup` becoming N+1** — mitigated by single-request-per-tick batching in `useNameResolver`; a vitest assertion asserts 50 mounts → 1 fetch. Backend caps batch at 256 to bound pathological cases.
4. **Display-font FOUC** — mitigated by `next/font/google` `display: 'swap'`, `--font-fraunces` CSS var, attaching `font-display` class only on h1/h2/display elements (small surface area), tuned `adjustFontFallback` against Inter.
5. **Light-theme contrast regressions** — mitigated by deliberate light-token authoring (not auto-inversion) and WCAG AA validation on every on-surface text + primary-on-primary-container button.
