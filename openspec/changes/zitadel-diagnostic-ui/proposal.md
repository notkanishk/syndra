## Why

The backend now exposes 12 discovery / management endpoints under `/api/v1/zitadel/*` plus a health probe, but there is no operator-facing surface to exercise them. During bring-up against a live Zitadel (`auth.example.org`), the only options are raw `curl` commands and log-tailing — slow, error-prone, and impossible to demo. The operator explicitly asked for a frontend panel that verifies the full management path (M2M token exchange, user/project discovery, role CRUD, grant CRUD) in one place.

Two gaps surfaced while planning this UI:

1. The proxy at `ui/src/app/api/proxy/[...path]/route.ts` only forwards `GET/POST/PUT`. Deleting a role or revoking a grant from the browser would silently fail with 405 from Next.js.
2. `/api/v1/zitadel/health` was gated by `withAPIKeyAuth` so it could be smoke-tested without an OIDC session — but the browser flow attaches the user's Zitadel JWT through the proxy, which the API-key middleware rejects. The health check is unusable from the UI.

## What Changes

* **New page `/zitadel` (admin-only)** — a single-route diagnostic console with four sections:
  * **M2M Health** — one-click full-path probe (key → JWT assertion → token exchange → Management API call) with latency and raw response.
  * **Projects & Roles** — pick a project, list its roles, create / rename / delete a role inline.
  * **Users & Grants** — pick a user, list their grants, assign / update / remove a grant inline.
  * **All Grants** — on-demand cross-user / cross-project grant overview.
* **Proxy DELETE** — `ui/src/app/api/proxy/[...path]/route.ts` gains a `DELETE` handler so role and grant deletion reach the backend.
* **Health auth unified** — `/api/v1/zitadel/health` moves from `withAPIKeyAuth` to `withOperatorAuth`, so the same admin JWT that gates the other `/zitadel/*` routes also gates health. The existing API-key cmdline path remains available in dev mode (where `withUserAuth` → `withAPIKeyAuth` fallback still kicks in).
* **Sidebar** — admin nav gains an "Operations" section with a "Zitadel Diagnostics" link.

No backend endpoints added. No database schema changes. No new dependencies.

## Capabilities

### Modified Capabilities

* `production-security-boundary` — the `/zitadel/*` namespace (including the health probe) is now uniformly gated by the admin JWT via `withOperatorAuth`, removing the parallel API-key shortcut that previously existed on `/zitadel/health`. Production mutations and diagnostics share one admin auth chain.

### Added Capabilities (toward Phase 5)

* `operational-readiness` — introduces the first operator-facing diagnostic surface in the admin UI: a single `/zitadel` page that exercises the live Zitadel management API (health probe, projects & roles CRUD, users & grants CRUD, cross-project grant view). This partially satisfies the P5 "operator visibility" goal without introducing full metrics / alerting.

## Impact

* New file: `ui/src/app/zitadel/page.tsx` (single self-contained client component).
* Modified: `ui/src/app/api/proxy/[...path]/route.ts` (DELETE handler).
* Modified: `ui/src/components/Sidebar.tsx` (new admin link).
* Modified: `backend/internal/handlers/router.go` (`/zitadel/health` swapped to `withOperatorAuth`).
* Zero new `go.mod` or `package.json` dependencies.
* No risk to existing endpoints — additive UI, auth model unified not relaxed.
