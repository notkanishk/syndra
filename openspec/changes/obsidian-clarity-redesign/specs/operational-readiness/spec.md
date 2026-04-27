> **Status:** Proposed | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Dashboard MUST consume the Obsidian Clarity design tokens

The dashboard MUST source every color, surface, font, radius, and shadow from the Obsidian Clarity token contract defined in `ui/src/app/globals.css`. Hard-coded color literals outside `globals.css` are forbidden so theme swaps and accessibility audits remain centralized.

#### Scenario: Token contract resolves both themes
- **WHEN** the `<html>` element carries `data-theme="dark"` or `data-theme="light"`
- **THEN** every documented Obsidian Clarity token (surface-container ladder, primary/secondary/tertiary, on-surface variants, outline + outline-variant, error + error-container) MUST resolve to the documented hex values
- **AND** the matching Tailwind utility (e.g. `bg-surface-container-high`, `text-on-surface-variant`, `border-outline`) MUST apply that token

#### Scenario: Light theme passes WCAG AA
- **WHEN** the dashboard renders under `data-theme="light"`
- **THEN** body text on every surface-container variant MUST meet WCAG AA at 4.5:1
- **AND** primary-on-primary-container button states MUST meet WCAG AA at 3:1 (large display) or 4.5:1 (body)

### Requirement: Live status MUST use the Pulse indicator

Live, in-flight, or attention-demanding states (sync intents, webhook retries, expiring grants, degraded data plane) MUST be represented by the `<Pulse/>` component (or `<Badge pulse/>` overlay) — not by static dots or by colored Badge text alone.

#### Scenario: Steady success uses non-animated pulse
- **WHEN** an item is healthy and live
- **THEN** the indicator MUST be a steady (non-animated) green dot via `<Pulse variant="success" static />`

#### Scenario: In-flight uses animated info pulse
- **WHEN** an item is processing or warming up (e.g. an in-flight provisioning intent)
- **THEN** the indicator MUST be an animated blue dot via `<Pulse variant="info" />`

#### Scenario: Degraded uses animated error pulse
- **WHEN** an item has failed or is degraded
- **THEN** the indicator MUST be an animated red dot via `<Pulse variant="error" />`

### Requirement: Client data fetching MUST go through TanStack Query

Every dashboard page MUST source its data via per-resource hooks under `ui/src/lib/queries/`. Direct `fetch("/api/proxy/...")` calls from page components are forbidden so request dedupe, caching, retry, and mutation invalidation are uniform.

#### Scenario: Pages compose query hooks
- **WHEN** a page needs backend data
- **THEN** it MUST call a hook from `ui/src/lib/queries/` (e.g. `useUsers`, `useBundles`, `useGovernanceSummary`)
- **AND** the page MUST NOT call `fetch()` directly against `/api/proxy/...`

#### Scenario: Mutations invalidate related query keys
- **WHEN** a mutation hook (e.g. `useCreateBundle`, `useApproveRequest`) succeeds
- **THEN** the related query keys MUST be invalidated so subsequent reads reflect the change
- **AND** a Sonner toast MUST surface the success or failure outcome

#### Scenario: One QueryClient per request in RSC
- **WHEN** the root layout renders on the server
- **THEN** the `getQueryClient()` helper MUST return a per-request `QueryClient` via `cache()` from `react`
- **AND** concurrent requests MUST NOT share QueryClient state

### Requirement: UID display surfaces MUST resolve to human names

Anywhere the dashboard renders an entity (user, project, role, bundle), the displayed string MUST be the resolved display name via the `<UserName/>`, `<ProjectName/>`, `<RoleName/>`, `<BundleName/>`, or `<ResourceName/>` components. Raw UUIDs are permitted only in detail panels behind a "Show ID" or `?debug=ids` affordance.

#### Scenario: Resolver batches resolution into one request per tick
- **WHEN** N Name components mount in the same animation frame
- **THEN** exactly one `POST /api/v1/lookup` request MUST be issued
- **AND** the response MUST be cached for 5 minutes so subsequent mounts hit cache

#### Scenario: Loading state is a skeleton placeholder
- **WHEN** a Name component is awaiting resolution
- **THEN** it MUST render a `<Skeleton/>` of approximate name width
- **AND** it MUST NOT render the raw UID in the meantime

#### Scenario: Miss falls back to a non-UID placeholder
- **WHEN** resolution returns no entry for the requested ID
- **THEN** the component MUST render the `fallback` prop (default `—`)
- **AND** the raw UID MUST be available via the `title` attribute (not visible in the layout)

### Requirement: POST /api/v1/lookup endpoint MUST exist and conform to the documented contract

The backend MUST expose `POST /api/v1/lookup` returning a partial-tolerant name map for users, projects, roles, and bundles. Frontend Name components depend on this endpoint exclusively.

#### Scenario: Empty arrays return empty maps
- **WHEN** a request body has all four arrays empty (or absent)
- **THEN** the response MUST be `200 OK` with `{users: {}, projects: {}, roles: {}, bundles: {}}`

#### Scenario: Partial miss returns the resolved subset
- **WHEN** a request body lists 3 user IDs and only 2 exist
- **THEN** the response MUST be `200 OK` with the 2 known IDs as keys; the unknown ID MUST be absent

#### Scenario: Oversized batch is rejected
- **WHEN** any of `user_ids`, `project_ids`, `role_keys`, `bundle_ids` exceeds 256 entries
- **THEN** the response MUST be `400 VALIDATION_FAILED` with a `details` field naming the offending array

#### Scenario: Strict body decoding rejects unknown fields
- **WHEN** a request body contains an unknown top-level field
- **THEN** the response MUST be `400` (via `decodeJSONStrict`)

#### Scenario: Auth required
- **WHEN** the request has no bearer token (and OIDC mode is configured)
- **THEN** the response MUST be `401 UNAUTHORIZED`

### Requirement: GET /api/v1/reconciliation/grants endpoint MUST exist for visibility

The backend MUST expose `GET /api/v1/reconciliation/grants` returning the diff between MkAuth direct grants and Zitadel grants. The endpoint is read-only; remediation is explicitly deferred.

#### Scenario: Drift cases are categorized
- **WHEN** an admin requests reconciliation
- **THEN** the response MUST contain three arrays: `only_in_mkauth`, `only_in_zitadel`, and `drift` (entries with role-set mismatches)
- **AND** each entry MUST include the user_id, project_id, and the role keys observed on each side

#### Scenario: Operator-level auth required
- **WHEN** the requester lacks the operator role
- **THEN** the response MUST be `403 FORBIDDEN`
- **AND** the response MUST NOT leak any drift data

### Requirement: Drawer primitive MUST share Modal a11y guarantees

The new `<Drawer/>` primitive MUST inherit the same focus-trap, `aria-modal`, Esc, and click-outside semantics as `<Modal/>`. Drawers used for audit detail and reconciliation drill-in MUST NOT bypass these affordances.

#### Scenario: Drawer traps focus
- **WHEN** a Drawer opens
- **THEN** focus MUST move into the first focusable element inside the Drawer
- **AND** Tab cycling MUST stay within the Drawer until it closes

#### Scenario: Drawer dismisses via Esc and click-outside
- **WHEN** a Drawer is open and the user presses Esc or clicks the scrim
- **THEN** the Drawer MUST close and focus MUST return to the trigger element

### Requirement: ConfirmModal a11y contract MUST be preserved through the Modal refactor

The `<ConfirmModal/>` refactor to compose the new generic `<Modal/>` MUST NOT regress focus trap, `aria-modal`, Esc, click-outside, or destructive-variant semantics. Tests for these contracts MUST be authored before the refactor lands.

#### Scenario: ConfirmModal tests precede refactor
- **WHEN** the ConfirmModal refactor commit is reviewed
- **THEN** the diff MUST include `Modal.test.tsx` and `ConfirmModal.test.tsx` introduced *before* the implementation change
- **AND** the tests MUST cover focus trap, Esc dismiss, click-outside dismiss, and destructive variant rendering

### Requirement: Operations and Grants admin routes MUST be admin-gated

The new `/operations` and `/grants` routes MUST verify operator-level session role server-side and MUST redirect non-admins to `/`. Sidebar links to these routes MUST NOT render for non-admin sessions.

#### Scenario: Non-admin redirected
- **WHEN** a non-admin session navigates to `/operations` or `/grants`
- **THEN** the RSC layer MUST redirect to `/` before rendering
- **AND** no operator data MUST appear in the response

#### Scenario: Admin sees Admin sidebar section
- **WHEN** an admin session loads the sidebar
- **THEN** an "Admin" eyebrow section MUST be visible with "Operations" and "Grants" links

#### Scenario: Non-admin sees no Admin section
- **WHEN** a non-admin session loads the sidebar
- **THEN** the "Admin" eyebrow section MUST NOT render
