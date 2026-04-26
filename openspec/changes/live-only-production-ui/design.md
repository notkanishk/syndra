# Live-Only Production UI Design

## Decisions

### 1. Catalog-driven form defaults instead of env-aware UI branches

The UI does not branch on `process.env.ZITADEL_DOMAIN` to decide what to render. The same code path runs in demo and live modes; what differs is the data flowing in. Initial form state is empty (`""`). Once the catalog/reference-data load resolves, defaults are derived from `catalog[0]` (or first user / first role). Submit buttons are disabled until both `project_id` and `role_key` are populated.

This avoids two failure modes that env-aware branches would invite: (a) demo-only paths becoming silently broken in production, (b) production-only paths losing test coverage in dev. With catalog-driven defaults, every commit exercises the live code path locally.

### 2. Authorship is derived from the authenticated principal, not the request body

`granted_by` and `reviewer_id` are no longer trusted from the request body in production. The backend resolves the actor from `getAdminUserID(ctx)` — populated by `withUserAuth` after Zitadel JWT validation. The body fields are kept as a `omitempty` fallback for demo/API-key mode where the context has no principal. The proxy route fills them with `session.id` for admin grant/decision flows in demo so audit attribution stays meaningful.

This is a per-request authorship guarantee: in production, the audit log records the JWT subject regardless of what the UI sent. In demo, the cookie session id wins. Tests pass unchanged because their requests carry the body field.

### 3. `system/mode` is auth-gated

Deployment posture (live vs demo, configured vs degraded) is sensitive enough that an unauthenticated probe should not see it. The endpoint is wrapped with `withCORS(withUserAuth(...))`, identical to other `/api/v1/*` routes. The UI fetches it once on every render of the sidebar through the existing `fetchWithAuth` helper.

### 4. The mode badge is silent in steady-state

Healthy live deployments render `null`. A small "Demo mode" outline tag shows in pure local-dev. A destructive-variant badge with hover tooltip shows on degraded fallback (configured-but-not-live). The chrome stays calm in normal operation; the safety net only fires when something is genuinely off.

Errors fetching `/system/mode` swallow silently to `null` — the badge must never break the layout.

### 5. `getSession()` rejects demo cookies in OIDC mode as defense-in-depth

A stale demo cookie left over from a local-dev session must never resolve into a production session. `getSession()` checks `process.env.ZITADEL_DOMAIN` and returns `null` when the cookie payload is `type: "demo"` and the env says we're live. Forces re-login through OIDC.

### 6. `EmptyState` copy lives at the call site

No central catalog of strings; each view writes its own title/description tailored to the view's domain. This keeps the wording specific and explanatory ("Once projects, applications, bundles, and mapping rules exist, they'll appear here as a connected graph") instead of generic ("Nothing here yet").

The component itself stays minimal: dashed border, optional icon and CTA, two tones (`neutral` and `destructive`).

### 7. Demo mode is preserved unchanged

This change is not a deprecation of demo mode. Local development without `ZITADEL_DOMAIN` still works end-to-end: login picks a demo identity, the directory serves the hardcoded catalog, the seed populates DB fixtures, audit attribution names the demo user (via proxy injection), and the new mode badge shows a small "Demo mode" outline indicator. The two leaks were demo data appearing in *production* — not demo mode itself.

## Non-decisions / Deferred

- A dashboard-level "live mode banner" beyond the small sidebar badge. The badge is intentionally quiet so admins focus on the actual UI; if more prominent surfacing is needed later, we'll add it as a follow-up.
- Backend rejection of `granted_by`/`reviewer_id` body fields outright (would break existing handler tests). The current "ctx-preferred, body-fallback" precedence captures the intent without test churn.
- A leak-test that asserts no `DEMO_USERS` strings appear in `LoginPage`'s rendered HTML when `ZITADEL_DOMAIN` is set. The current vitest setup runs in `node` env without React DOM testing utilities. The structural fix (extracting `<DemoIdentityCard>` and runtime-guarding it) is sufficient; we'll add the rendering test alongside the broader React Testing Library setup planned in `dashboard-ux-elevation`.
