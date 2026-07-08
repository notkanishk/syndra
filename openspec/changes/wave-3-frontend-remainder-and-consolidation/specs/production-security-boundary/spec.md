> **Status:** Proposed | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: The admin/member routing and proxy boundary MUST be regression-tested

The admin/member split is enforced in two places — the Next.js `middleware.ts` (route gating) and the `/api/proxy/[...path]` route (backend-call gating). These enforcements MUST carry automated tests so a regression is caught before deploy, not in production.

#### Scenario: Member is redirected away from admin-only routes
- **WHEN** a session with `role: "user"` requests an admin-only path (applications, audit, bundles, graph, policies, projects, users)
- **THEN** `middleware.ts` redirects to `/`

#### Scenario: Stale demo cookie is cleared in production mode
- **WHEN** `ZITADEL_DOMAIN` is set and a request carries a demo/legacy session cookie
- **THEN** `middleware.ts` redirects to `/login` and clears the cookie (maxAge 0)

#### Scenario: Member proxy calls are self-scoped and allowlisted
- **WHEN** a `role: "user"` session calls the proxy for `users/{ownId}/grants`
- **THEN** the call is permitted
- **WHEN** the same session calls `users/{otherId}/grants` or any non-allowlisted path
- **THEN** the proxy returns 403
- **AND** a member GET is limited to `catalog`, `applications`, `requests`, or self-scoped paths, and a member POST is limited to `requests`

#### Scenario: Member request writes are pinned to the caller
- **WHEN** a `role: "user"` session POSTs an access request through the proxy
- **THEN** `requester_id` is forced to the session's own id
- **AND** a member's `requests` list response is filtered to rows where `requester_id` equals the session id

### Requirement: Every admin page MUST be gated by one enforcement mechanism, and both mechanisms MUST be tested

Every admin page MUST be gated by exactly one of two mechanisms: the `middleware.ts` `ADMIN_ONLY_PATHS` list (redirect before render) or a page-level server guard (`getSession()` → `redirect`). No admin page may be left gated by neither. Both mechanisms MUST carry regression tests so a page cannot silently ship ungated (as `/zitadel` did before this change).

#### Scenario: Middleware-gated admin paths redirect members
- **WHEN** a `role: "user"` session requests any path in `ADMIN_ONLY_PATHS`
- **THEN** middleware redirects to `/`

#### Scenario: Page-gated admin routes redirect members and the unauthenticated
- **WHEN** a non-admin session invokes a page-gated admin route (`/operations`, `/operations/cascades`, `/governance/pending`, `/governance/drift`, `/grants`, `/zitadel`)
- **THEN** the page's server guard calls `redirect("/")`
- **AND** when there is no session, it calls `redirect("/login")`
- **AND** the client island never hydrates for a non-admin

#### Scenario: The Zitadel diagnostics surface is admin-gated
- **WHEN** a `role: "user"` session navigates to `/zitadel`
- **THEN** the server guard redirects to `/` before the diagnostics surface hydrates (previously it rendered, protected only by proxy 403s on its data calls)
