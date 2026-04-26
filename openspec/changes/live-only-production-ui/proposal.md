## Why

The `live-zitadel-data-source` and `live-directory-identity-completeness` changes flipped the backend's directory layer to live Zitadel when configured. But the **frontend still leaked demo identifiers into production**, and live deployments now hit empty/sparse states the UI never planned for. Concretely:

1. **Login page leak.** `ui/src/app/login/page.tsx:13` called `getDemoUsers()` unconditionally; the demo identity catalog (Alice Rivera, Sam Patel, Maya Chen, Leo Brooks, Ava Morgan with their teams and emails) was serialized into the RSC payload of every production login render even though the JSX hid the demo card.
2. **Hardcoded form defaults.** Four call sites (`bundles/page.tsx`, `users/page.tsx`, `requests/AdminRequestsView.tsx`, `requests/UserRequestsView.tsx`) shipped `project_id: "printing"`, `role_key: "member"`, `requester_id: "ava_guest"`, `granted_by: "alice.rivera"`, `reviewer_id: "alice.rivera"` as initial state — invalid in live mode and a 400 on early submission.
3. **Authorship from request body.** Backend handlers accepted `granted_by` and `reviewer_id` from the request body and the UI sent hardcoded literals; admin-action audit attribution was a UI lie.
4. **No detection of unexpected demo fallback.** If `ZITADEL_DOMAIN` is set but the management key is missing/unreadable, `directory.Init` quietly falls back to the demo source. There was no signal to admins that they were looking at fixture data.
5. **Empty-state UX.** Most dashboard views rendered a blank grid or terse `<p>Loading…</p>` when live data was empty — fine with the seeded demo catalog, jarring against an empty production directory.
6. **Misleading copy.** The admin overview talked about "demo data spanning…", "seeded personas", and "seeded data for every major admin view" — accurate only in local dev.

## What Changes

- **Login page.** Extract `<DemoIdentityCard>` into its own component that calls `getDemoUsers()` only when rendered. Branch JSX so the demo card mounts only when `!isOidcMode`. Add a runtime guard `if (process.env.ZITADEL_DOMAIN) return null` inside the demo card as defense-in-depth.
- **Catalog-driven form defaults.** Replace the four hardcoded literal defaults with empty initial state. After the catalog/reference-data load resolves, derive defaults from `catalog[0]` / `catalog.users[0]`. Disable submit buttons until the project_id and role_key fields are populated. Same code path runs in demo and live modes.
- **Authorship from auth principal.** Backend handlers now derive `granted_by`/`reviewer_id` from the authenticated subject (Zitadel JWT `sub` via `getAdminUserID(ctx)`), with a body-field fallback for demo/local-dev where API-key auth has no principal. UI components stop sending hardcoded user IDs. The proxy injects `session.id` for admin grant/decision requests in demo mode so audit attribution stays meaningful.
- **`/api/v1/system/mode` diagnostic.** New auth-gated endpoint reporting `{directory, seed_active, zitadel_configured, degraded}`. `directory.Default.Tag()` already exists; `seed.DemoSeedActive()` is a new thread-safe getter. Frontend renders a `SystemModeBadge` in the sidebar footer: silent in healthy live mode, prominent destructive badge when configured-but-degraded, small outline tag in pure local-dev demo mode.
- **`SessionCookie` defense in depth.** `getSession()` now returns `null` when `ZITADEL_DOMAIN` is set but the cookie payload is `type: "demo"`, so a stale demo cookie can never resolve into a production session.
- **Empty-state pass.** Add reusable `<EmptyState>` component (dashed border, explanatory copy, optional CTA). Apply across all seven dashboard views: admin overview activity card, member service catalog, users list, projects, applications + Token Simulator, bundles, mapping rules (migrating the existing inline empty), access requests (admin and user views), audit feed, governance watchlist, and topology graph.
- **Neutral copy.** Rewrite three "demo data / seeded" lines on the admin overview into copy that's accurate in both modes.

## Capabilities

### Modified Capabilities

- `demo-catalog`: production UI MUST NOT serialize demo catalog entities (identities, project IDs, role keys) into HTML, RSC payloads, or form defaults when `ZITADEL_DOMAIN` is set.
- `user-management`: empty user directory MUST render an explanatory empty state. Grant authorship MUST come from the authenticated principal, not from the request body.
- `application-claims`: Token Simulator MUST handle zero applications gracefully. Access-request authorship MUST come from the authenticated principal.
- `service-catalog`: member portal MUST render an explanatory empty state when no services are available.
- `operational-readiness`: system MUST expose a directory-mode diagnostic at `GET /api/v1/system/mode` and the UI chrome MUST surface unexpected fallback to admins.

## Impact

- **No migration required.** Behavior changes only.
- **Backend handler change** is backwards-compatible: body-field `granted_by`/`reviewer_id` are still accepted as fallbacks, so existing tests pass unchanged. Production clients (the UI) stop sending them.
- **Created**: `backend/internal/handlers/{system.go, system_test.go}`, `ui/src/components/{SystemModeBadge.tsx, ui/EmptyState.tsx}`.
- **Modified**: `backend/internal/handlers/{router.go, access.go}`, `backend/internal/seed/demo.go`, `ui/src/app/login/page.tsx`, `ui/src/lib/{session.ts, api.ts}`, `ui/src/components/Sidebar.tsx`, all dashboard `page.tsx` views and the two requests components, `ui/src/app/api/proxy/[...path]/route.ts`, `ui/src/lib/__tests__/session.test.ts`.
- **Demo mode unchanged.** With `ZITADEL_DOMAIN` unset, the login page still shows demo identities, the form defaults still populate (now from the catalog endpoint instead of hardcoded literals), audit attribution still names the demo session user (proxy-injected), and the new `SystemModeBadge` shows a small "Demo mode" outline tag.
