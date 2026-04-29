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

### Requirement: Zitadel diagnostics MUST surface live status via a polling Pulse tile

The `/zitadel` page MUST render a top-level glass-card "Live status" tile that polls `useZitadelHealth()` (`GET /zitadel/health`) every 10 seconds and surfaces a `<Pulse/>` whose variant maps to the backend's response: steady-success when `status === "ok"`, warn-pulse when `status === "disabled"` (locally configured but not exercising the management API), and error-pulse when `status === "error"` or the proxy is unreachable.

The polling cadence MUST pause when the tab is hidden so background polling cost stays bounded. The deeper CRUD sections (Projects, Users, Grants, Rotation) MAY continue to use the existing imperative `apiGet` / `apiSend` shape — full hook migration of those sections is out of stage scope and tracked separately.

#### Scenario: Steady-state success
- **WHEN** the `/zitadel` health probe returns `{status: "ok"}`
- **THEN** the Live Status tile MUST render `<Pulse variant="success" static/>` (non-animating green)
- **AND** the tile body MUST display "Connected"

#### Scenario: Disabled or local-only mode
- **WHEN** the health probe returns `{status: "disabled"}` (no Zitadel domain configured, local-policy-only mode)
- **THEN** the tile MUST render `<Pulse variant="warn"/>` (animating amber)
- **AND** the tile body MUST display "Disabled (local-policy-only)"

#### Scenario: Backend unreachable
- **WHEN** the health probe returns `{status: "error"}` or the request fails at the transport layer
- **THEN** the tile MUST render `<Pulse variant="error"/>` (animating red)
- **AND** the error detail (if any) MUST appear below the tile in the `var(--error)` tone

### Requirement: Topology canvas pan/zoom MUST be preserved through the Stage 3 reskin

The `/graph` page MUST preserve the existing pan/zoom mechanics (drag to pan, ⌘/Ctrl + scroll to zoom, range clamped to [0.4, 2.5], reset button) verbatim. The Stage 3 changes are restricted to chrome: the inspector moves into a glass `<Drawer/>` and the legend pills migrate to a floating top-left glass chip. Node deeplinks MUST continue to route to the corresponding capability surface (`/applications`, `/bundles`, `/projects`).

#### Scenario: Click node opens Drawer
- **WHEN** an admin clicks a node in the topology canvas
- **THEN** the inspector Drawer MUST slide in from the right (`<Drawer size="lg"/>`)
- **AND** the Drawer MUST render the node's metadata, connected edges, and a "View details →" deeplink to the appropriate capability surface
- **AND** the Drawer MUST follow the same focus-trap, Esc, and click-outside semantics as `<Modal/>`

#### Scenario: Pan/zoom unchanged
- **WHEN** an admin drags the empty canvas surface or scrolls with ⌘/Ctrl held
- **THEN** the canvas MUST pan or zoom respectively
- **AND** the zoom level MUST remain clamped to [0.4, 2.5]
- **AND** the "Reset" button MUST restore `{x: 0, y: 0, scale: 1}`

### Requirement: Mapping rule authoring MUST happen inside a Modal

The `/policies` page's CreateRuleForm MUST render inside a `<Modal/>` (focus-trap, Esc, click-outside). The form MUST debounce a `useValidateMappingRule` mutation and display cycle / self-reference warnings inline before the operator can create the rule. Rules with `would_cycle` or `self_reference` set to true MUST disable the submit button.

#### Scenario: Modal-gated authoring
- **WHEN** an admin clicks the "+ New rule" toolbar button
- **THEN** a `<Modal/>` MUST open with the title "Create mapping rule"
- **AND** the modal MUST follow the focus-trap, Esc, and click-outside contract from `<Modal/>`

#### Scenario: Cycle warning blocks submit
- **WHEN** the validation result returns `would_cycle: true` or `self_reference: true`
- **THEN** the "Create rule" submit button MUST be disabled
- **AND** a warning MUST render in the live preview panel inside the modal

### Requirement: Bundle and role authoring MUST happen inside a Modal

Bundle creation, the add-roles-to-bundle picker, and role creation MUST each render inside `<Modal/>` (focus trap, Esc, click-outside, glass-card body) rather than as inline page forms. The bundles toolbar MUST expose `+ Create role` and `+ Create bundle` buttons that open `<CreateRoleModal/>` and `<CreateBundleModal/>` respectively. A bundle row's "Manage roles" action MUST open `<AddRolesToBundlePicker/>` for the selected bundle. Sonner MUST surface every mutation outcome.

#### Scenario: Create bundle is gated by Modal
- **WHEN** an admin clicks "+ Create bundle" on `/bundles`
- **THEN** a `<Modal/>` MUST open with the title "Create a role bundle"
- **AND** the form MUST POST `/api/v1/bundles` with `{name, description}`
- **AND** the modal MUST close and a Sonner success toast MUST surface on 201 Created

#### Scenario: Add roles to bundle is gated by Modal
- **WHEN** an admin clicks "Manage roles" on an expanded bundle
- **THEN** an `<AddRolesToBundlePicker/>` Modal MUST open scoped to that bundle
- **AND** the picker MUST list every role from `GET /api/v1/roles` grouped by project
- **AND** roles already in the bundle MUST be disabled with "Already in bundle" copy
- **AND** the picker MUST support multi-select; Confirm MUST issue a sequential `POST /api/v1/bundles/{id}/roles` for each selection
- **AND** any failure MUST stop the loop and keep the un-added selections in the picker for retry

#### Scenario: Create role is gated by Modal
- **WHEN** an admin clicks "+ Create role" on `/bundles`
- **THEN** a `<Modal/>` MUST open with the title "Create a project role"
- **AND** the role_key field MUST auto-derive from the display name (lowercase, `[a-z0-9_-]`) until the operator manually edits it
- **AND** a "Clone from" select MUST list existing roles in the selected project so the new role can inherit display name and description
- **AND** a 409 CONFLICT response MUST surface inline (field-level error on role_key) without closing the modal or invoking the success path

### Requirement: Operations page MUST surface live operator queues

The new `/operations` route MUST render three tabbed queues — Intents, Webhook events, Onboarding triggers — each backed by its own polling hook (`useIntents`, `useWebhookEvents`, `useOnboardingTriggers`) refreshing every 5 seconds and pausing while the tab is hidden. Every row MUST carry a status `<Pulse/>` whose variant follows the agreed mapping (success/warn/error/info), surface the target identity via `<UserName/>` and `<ProjectName/>` where applicable, show the relative age, truncate the last error message with full text on hover, and expose a "Payload" button that opens the raw record inside a `<Modal/>` with `<JsonView/>`.

#### Scenario: Tabs render live data
- **WHEN** an admin opens `/operations`
- **THEN** three role-tab buttons MUST render with the labels Intents / Webhook events / Onboarding triggers
- **AND** the Intents tab MUST be the default selection
- **AND** the rows MUST poll on a 5-second cadence via React Query

#### Scenario: Status filter pills (intents only)
- **WHEN** the Intents tab is active
- **THEN** an "All / pending / in_flight / succeeded / failed" pill row MUST render
- **AND** selecting a pill MUST refetch with `?status=` so the filter is server-honored
- **AND** webhook and onboarding tabs MUST NOT show the pill row (their backends do not support `?status=` filtering uniformly)

#### Scenario: Payload modal
- **WHEN** an admin clicks "Payload" on any row
- **THEN** a `<Modal/>` MUST open with the row's full record rendered via `<JsonView/>`
- **AND** the modal MUST follow the focus-trap, Esc, and click-outside contract from `<Modal/>`

#### Scenario: No raw UUID escapes operator rows
- **WHEN** any row with a user_id or project_id renders
- **THEN** the visible label MUST resolve through `<UserName/>` and `<ProjectName/>` once the lookup batch settles
- **AND** the raw UUID MUST appear only via the "Payload" `<JsonView/>` drill-in or a `?debug=ids` flag

### Requirement: Grants page MUST present cross-source ledger and reconciliation diff

The new `/grants` route MUST render two tabs: **All grants** (a unioned ledger sourced from `GET /api/v1/zitadel/grants` and `GET /api/v1/reconciliation/grants`) and **Reconciliation** (a drift snapshot). All rows on both tabs MUST resolve user/project/role names via the Name components — never raw UUIDs. The page is read-only; no remediation, sync, or apply actions MAY appear on either tab per the obsidian-clarity-redesign visibility-only mandate.

#### Scenario: All grants source pills
- **WHEN** the All grants tab renders
- **THEN** every row MUST carry a Source pill ("MkAuth + Zitadel", "Zitadel only", "Derived from rule", or "MkAuth only (sync gap)")
- **AND** "Derived from rule" MUST be assigned when a mapping rule's target equals the row's `(project_id, role_key)` and the same pair is absent from MkAuth direct grants
- **AND** "MkAuth only (sync gap)" MUST be assigned when the (user, project, role) is present in the MkAuth `direct_role_grants` table but absent from the Zitadel-side grant

#### Scenario: All grants filter rail
- **WHEN** the All grants tab renders
- **THEN** a sticky filter rail MUST surface user-text, source pills, and project pills
- **AND** activating a filter MUST narrow the visible row set client-side without refetching
- **AND** a "Clear filters" affordance MUST appear when any filter is active

#### Scenario: Reconciliation drift summary
- **WHEN** the Reconciliation tab renders
- **THEN** three count cards MUST display Role mismatch, Only in MkAuth, and Only in Zitadel
- **AND** clicking a card MUST scope the table below to that drift category
- **AND** a green "in sync" message MUST replace the lists when all three counts are zero

#### Scenario: Reconciliation drawer drill-in
- **WHEN** an admin clicks a drift row
- **THEN** a `<Drawer size="lg"/>` MUST slide in from the right
- **AND** for role-mismatch entries the Drawer MUST render the MkAuth-side and Zitadel-side records side-by-side via `<JsonView/>`
- **AND** the Drawer MUST follow the same focus-trap, Esc, and click-outside semantics as `<Modal/>`

#### Scenario: No remediation actions present
- **WHEN** either tab renders
- **THEN** the surface MUST NOT contain "Apply", "Sync", or any other remediation button
- **AND** auto-correction is explicitly deferred to a later change (Phase 5/6 reconciliation engine)
