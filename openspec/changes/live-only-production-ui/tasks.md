# Tasks

## 1. Backend — system/mode endpoint and authorship from auth

- [x] 1.1 Add `seed.DemoSeedActive() bool` getter (atomic flag set during `EnsureDemoData`).
- [x] 1.2 Create `backend/internal/handlers/system.go` with `handleSystemMode`, returning `{directory, seed_active, zitadel_configured, degraded}`. Inject `directorySource`, `seedActive`, `systemZitadelConfigured` for testability.
- [x] 1.3 Wire `GET /api/v1/system/mode` in `router.go` next to `/healthz`, gated by `withCORS(withUserAuth(...))`.
- [x] 1.4 In `handlers/access.go`, add `resolveActor(r, bodyValue)` helper. Prefer `getAdminUserID(ctx)`, fall back to body field, final fallback "system".
- [x] 1.5 Use `resolveActor` for `granted_by` in `handleUpsertUserDirectGrant`.
- [x] 1.6 Use `resolveActor` for `reviewer_id` in `handleResolveAccessRequest`. Validate "approved" requires a non-system reviewer.
- [x] 1.7 Use `resolveActor` for the audit actor in `handleCreateAccessRequest` (admin-on-behalf attribution).
- [x] 1.8 Mark `GrantedBy`/`ReviewerID` request fields as `omitempty` and document them as demo fallbacks.

## 2. Backend — tests

- [x] 2.1 `backend/internal/handlers/system_test.go` — three scenarios (live, demo unconfigured, configured+degraded). Uses a `stubSource` and injectable hooks.
- [x] 2.2 Existing `access_flow_test.go` continues to pass without modification (resolveActor honors the body fallback when ctx is empty).

## 3. Frontend — leak fixes

- [x] 3.1 `ui/src/app/login/page.tsx` — extract `<DemoIdentityCard>` component; call `getDemoUsers()` only inside the demo branch. Add runtime guard on the demo card.
- [x] 3.2 `ui/src/app/bundles/page.tsx` — empty initial `newRole` state; populate from catalog on load; disable "Add Role" until populated.
- [x] 3.3 `ui/src/app/users/page.tsx` — empty initial `grantForm`; populate from catalog on load; remove `granted_by` from request body; disable Save until populated.
- [x] 3.4 `ui/src/components/requests/AdminRequestsView.tsx` — empty initial `form`; populate from catalog (users + projects); remove `reviewer_id` from decision body; disable Create until populated.
- [x] 3.5 `ui/src/components/requests/UserRequestsView.tsx` — empty initial `form`; existing post-load backfill now wins; disable Submit until populated.
- [x] 3.6 `ui/src/app/page.tsx` — replace three "demo data / seeded personas / seeded data" copy lines with neutral copy.
- [x] 3.7 `ui/src/lib/session.ts` — defense-in-depth: `getSession()` returns `null` when `ZITADEL_DOMAIN` is set and cookie is `type: "demo"`.

## 4. Frontend — system/mode badge

- [x] 4.1 Add `fetchSystemMode()` and `SystemMode` type to `ui/src/lib/api.ts`.
- [x] 4.2 Create `ui/src/components/SystemModeBadge.tsx` (server component) with the three render rules. Errors swallow to `null`.
- [x] 4.3 Mount the badge in `ui/src/components/Sidebar.tsx` footer.

## 5. Frontend — proxy demo-actor injection

- [x] 5.1 Update `ui/src/app/api/proxy/[...path]/route.ts` to inject `granted_by` (for `POST /users/{id}/grants`) and `reviewer_id` (for `POST /requests/{id}/decision`) from `session.id` when the admin session is `demo`. OIDC sessions skip injection — the backend resolves from JWT.

## 6. Frontend — tests

- [x] 6.1 Extend `ui/src/lib/__tests__/session.test.ts` with `getSession()` scenarios: demo cookie under demo env resolves; demo cookie under OIDC env returns null; OIDC cookie resolves regardless; missing cookie returns null. Mocks `next/headers` with a settable cookie value.

## 7. Frontend — empty-state pass

- [x] 7.1 Create `ui/src/components/ui/EmptyState.tsx` (dashed border, optional icon and CTA, two tones).
- [x] 7.2 Apply across all dashboard views: admin overview activity card; member service catalog; users list; projects; applications + Token Simulator (no-apps state); bundles; mapping rules (migrate existing inline empty state); admin and user request queues; audit feed; governance watchlist (expiring grants and cleanup hints sub-states); topology graph.

## 8. OpenSpec deltas

- [x] 8.1 `specs/demo-catalog/spec.md` — add requirement: production UI MUST NOT serialize demo entities.
- [x] 8.2 `specs/user-management/spec.md` — empty user directory empty state; grant authorship from auth principal.
- [x] 8.3 `specs/application-claims/spec.md` — Token Simulator empty state; access-request authorship from auth principal.
- [x] 8.4 `specs/service-catalog/spec.md` — member portal empty state.
- [x] 8.5 `specs/operational-readiness/spec.md` — `/api/v1/system/mode` endpoint with three-mode scenarios.

## 9. Validation

- [x] 9.1 `cd backend && go vet ./... && go test ./...` — 215 tests pass.
- [x] 9.2 `cd ui && bun run test && bun run lint && bun run build` — 10 tests pass, lint clean, build clean.
- [x] 9.3 `openspec validate live-only-production-ui --strict` — passes.
- [ ] 9.4 Codebase memory: `mcp__codebase-memory-mcp__detect_changes` and re-index after merge.
